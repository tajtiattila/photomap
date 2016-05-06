package imagecache

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/tajtiattila/basedir"
	"github.com/tajtiattila/photomap/source"
)

type ImageInfo struct {
	// key for thumb/icon lookup
	Id string `json:"id"`

	// image dimensions
	Width  int `json:"w,omitempty"`
	Height int `json:"h,omitempty"`

	// gps position
	Lat  float64 `json:"lat,omitempty"`
	Long float64 `json:"long,omitempty"`
}

type ImageCache struct {
	src source.ImageSource
	db  *leveldb.DB

	// keysrcid images are read-only after initialized in init()
	keysrcid map[string]string
	images   []ImageInfo

	pimtx sync.Mutex // protects pigen
	pigen map[string]*genPhotoIcon
}

func New(src source.ImageSource) (*ImageCache, error) {
	cachedir, err := basedir.Cache.EnsureDir("PhotoMap", 0700)
	if err != nil {
		return nil, err
	}
	db, err := leveldb.OpenFile(filepath.Join(cachedir, "imagecache.leveldb"), nil)
	if err != nil {
		return nil, err
	}
	ic := &ImageCache{
		src:      src,
		db:       db,
		keysrcid: make(map[string]string),
		pigen:    make(map[string]*genPhotoIcon),
	}
	return ic, ic.init()
}

func (ic *ImageCache) Images() []ImageInfo {
	return ic.images
}

func (ic *ImageCache) Close() error {
	ic.src.Close()
	return ic.db.Close()
}

func (ic *ImageCache) PhotoIcon(key string) (image.Image, error) {
	ic.pimtx.Lock()
	pg, ok := ic.pigen[key]
	if !ok {
		pg = new(genPhotoIcon)
		ic.pigen[key] = pg
	}
	ic.pimtx.Unlock()

	pg.once.Do(func() {
		var err error
		pg.im, err = ic.genPhotoIcon(pg, key)
		if err != nil {
			log.Println(err)
		}
	})

	if pg.im == nil {
		return nil, fmt.Errorf("missing thumb for %s", key)
	}
	return pg.im, nil
}

func (ic *ImageCache) genPhotoIcon(pg *genPhotoIcon, key string) (image.Image, error) {
	k := []byte(photoIconPfx + key)
	data, err := ic.db.Get(k, nil)
	switch err {
	case nil:
		// return cached image
		im, _, err := image.Decode(bytes.NewReader(data))
		return im, err
	case leveldb.ErrNotFound:
		// pass
	default:
		return nil, err
	}

	log.Println("thumbing", ic.keysrcid[key])

	// todo: sync.Once
	rc, err := ic.src.Open(ic.keysrcid[key])
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	// generate thumb
	t := NewThumber()
	im, err := t.PhotoIcon(rc)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if err := png.Encode(buf, im); err != nil {
		log.Println("can't encode thumb for cache:", err)
		return im, err
	}
	if err := ic.db.Put(k, buf.Bytes(), nil); err != nil {
		log.Println("can't save thumb to cache:", err)
	}

	return im, err
}

func (ic *ImageCache) init() error {
	mmt, err := ic.src.ModTimes()
	if err != nil {
		return err
	}

	for srcid, mt := range mmt {
		key, err := ic.getKey(srcid)
		if err != nil {
			return err
		}
		ic.keysrcid[key] = srcid
		ce, err := ic.loadFreshCacheEntry(key, srcid, mt)
		if err != nil {
			return err
		}
		if !ce.IsErr {
			ic.images = append(ic.images, ce.ImageInfo)
		}
	}

	return nil
}

const (
	imageInfoPfx = "imageinfo|"
	photoIconPfx = "photoicon|"
	thumbPfx     = "thumb|"
)

// load cache entry and refresh if needed
func (ic *ImageCache) loadFreshCacheEntry(key, srcid string, mt time.Time) (cacheEntry, error) {
	k := []byte(imageInfoPfx + key)
	data, err := ic.db.Get(k, nil)
	if err == nil {
		var ce cacheEntry
		if err = json.Unmarshal(data, &ce); err != nil {
			return cacheEntry{}, err
		}
		if ce.SrcId == srcid && ce.ModTime.Before(mt) {
			// cache up to date
			return ce, nil
		}
		err = ic.db.Delete([]byte(photoIconPfx+key), nil)
		if err != nil && err != leveldb.ErrNotFound {
			log.Printf("delete cached photo icon %q/%q: %v", key, srcid, err)
		}
		err = ic.db.Delete([]byte(thumbPfx+key), nil)
		if err != nil && err != leveldb.ErrNotFound {
			log.Printf("delete cached thumb %q/%q: %v", key, srcid, err)
		}
	}
	ii, err := ic.src.Info(srcid)
	ce := cacheEntry{SrcId: srcid}
	if err != nil {
		ce.IsErr = true
	} else {
		ce.ImageInfo = ImageInfo{
			Id:     key,
			Width:  ii.Width,
			Height: ii.Height,
			Lat:    ii.Lat,
			Long:   ii.Long,
		}
	}
	data, err = json.Marshal(ce)
	if err != nil {
		panic("can't marshal cacheEntry")
	}
	return ce, ic.db.Put(k, data, nil)
}

func (ic *ImageCache) getKey(srcid string) (string, error) {
	k := append([]byte("key|"), srcid...)
	data, err := ic.db.Get(k, nil)
	if err == nil {
		return string(data), nil
	}
	if err != leveldb.ErrNotFound {
		return "", err
	}

	hash := sha1.Sum([]byte(srcid))
	h := hash[:9]
	enc := base64.RawURLEncoding
	prefix := []byte(imageInfoPfx)
	key := make([]byte, len(prefix)+enc.EncodedLen(len(h)))
	copy(key, prefix)
	for {
		enc.Encode(key[len(prefix):], h)
		has, err := ic.db.Has(key, nil)
		if err != nil {
			return "", err
		}
		if !has {
			// key not in use yet
			err = ic.db.Put(k, key[len(prefix):], nil)
			return string(key[len(prefix):]), err
		}
		incByteArray(h)
	}
}

func incByteArray(p []byte) {
	for i := len(p) - 1; i >= 0; i-- {
		p[i]++
		if p[i] != 0 {
			return
		}
	}
}

type cacheEntry struct {
	SrcId string

	ModTime time.Time
	ImageInfo
	IsErr bool
}

type genPhotoIcon struct {
	once sync.Once
	im   image.Image
}
