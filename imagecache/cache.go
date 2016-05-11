package imagecache

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
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

	CreateTime time.Time

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

	photoIconMtx sync.RWMutex // protects photoIcon
	photoIcon    map[string]cachedImage
	photoIconGen *parallelGroup

	thumbGen *parallelGroup
}

// cachedImage is an image or an error
// that occurred while generating it
type cachedImage struct {
	im  image.Image
	err error
}

type cachedData struct {
	raw []byte
	err error
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

		photoIcon: make(map[string]cachedImage),
	}
	ic.photoIconGen = newParallelGroup(4)
	ic.thumbGen = newParallelGroup(4)
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
	ic.photoIconMtx.RLock()
	cim, ok := ic.photoIcon[key]
	ic.photoIconMtx.RUnlock()
	if ok {
		return cim.im, cim.err
	}

	im, err := ic.photoIconGen.Do(key, func() (interface{}, error) {
		return ic.createPhotoIcon(key)
	})
	if im != nil {
		cim.im = im.(image.Image)
	}
	cim.err = err

	ic.photoIconMtx.Lock()
	ic.photoIcon[key] = cim
	ic.photoIconMtx.Unlock()

	return cim.im, cim.err
}

func (ic *ImageCache) Thumbnail(key string) (io.ReadSeeker, time.Time, error) {
	data, err := ic.thumbnail(key)
	if err != nil {
		return nil, time.Time{}, err
	}
	var mt time.Time
	if len(data) > 8 {
		mt = time.Unix(int64(binary.BigEndian.Uint64(data)), 0)
		data = data[8:]
	}
	return bytes.NewReader(data), mt, nil
}

func (ic *ImageCache) thumbnail(key string) ([]byte, error) {
	data, err := ic.db.Get([]byte(thumbPfx+key), nil)
	if err == nil {
		return data, nil
	}
	if err != leveldb.ErrNotFound {
		return nil, err
	}

	di, err := ic.thumbGen.Do(key, func() (interface{}, error) {
		return ic.createThumb(key)
	})
	if err != nil {
		return nil, err
	}
	return di.([]byte), nil
}

func (ic *ImageCache) init() error {
	for srcid, mt := range ic.src.ModTimes() {
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
		delPfx := []string{photoIconPfx, thumbPfx}
		for _, dp := range delPfx {
			err = ic.db.Delete([]byte(dp+key), nil)
			if err != nil && err != leveldb.ErrNotFound {
				log.Printf("delete from cache %q/%q: %v", key, srcid, err)
			}
		}
	}
	ii, err := ic.src.Info(srcid)
	ce := cacheEntry{SrcId: srcid}
	if err != nil {
		ce.IsErr = true
	} else {
		ce.ImageInfo = ImageInfo{
			Id:         key,
			CreateTime: ii.CreateTime,
			Width:      ii.Width,
			Height:     ii.Height,
			Lat:        ii.Lat,
			Long:       ii.Long,
		}
	}
	data, err = json.Marshal(ce)
	if err != nil {
		panic("can't marshal cacheEntry")
	}
	return ce, ic.db.Put(k, data, nil)
}

func (ic *ImageCache) createPhotoIcon(key string) (image.Image, error) {
	im, err := ic.loadImage(photoIconPfx + key)
	if err == nil {
		return im, nil
	}
	if err != leveldb.ErrNotFound {
		log.Printf("createPhotoIcon db get %q: %v", key, err)
		return nil, err
	}

	rc, err := ic.src.Open(ic.keysrcid[key])
	if err != nil {
		log.Printf("createPhotoIcon read %q: %v", key, err)
		return nil, err
	}
	defer rc.Close()

	im, err = source.LoadImage(rc)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	im = MakeScaler(20, 20).Scale(im)

	// add frame
	im = Frame(im, 2, color.RGBA{255, 255, 255, 255})

	// add shadow
	shadow := Shadow{
		Color: color.RGBA{0, 0, 0, 128},
		Dx:    0,
		Dy:    1,
		Blur:  4,
	}

	im = shadow.Apply(im)

	ic.storeImage(photoIconPfx+key, im, png.Encode)

	return im, nil
}

// generate thumb for key, store it in db, and return the new
// image encoded as jpeg
func (ic *ImageCache) createThumb(key string) ([]byte, error) {
	rc, err := ic.src.Open(ic.keysrcid[key])
	if err != nil {
		log.Printf("source read %q: %v", key, err)
		return nil, err
	}
	defer rc.Close()

	im, err := source.LoadImage(rc)
	if err != nil {
		log.Printf("source decode %q: %v", key, err)
		return nil, err
	}

	im = MakeScaler(100, 100).Scale(im)

	mt := make([]byte, 8)
	binary.BigEndian.PutUint64(mt, uint64(time.Now().Unix()))

	buf := new(bytes.Buffer)
	buf.Write(mt)

	if err := jpeg.Encode(buf, im, nil); err != nil {
		log.Printf("thumb encode %q: %v", key, err)
		return nil, err
	}

	if err := ic.db.Put([]byte(thumbPfx+key), buf.Bytes(), nil); err != nil {
		log.Println("can't store image in cache:", err)
	}
	return buf.Bytes(), nil
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

func (ic *ImageCache) loadImage(k string) (image.Image, error) {
	data, err := ic.db.Get([]byte(k), nil)
	if err != nil {
		return nil, err
	}
	im, _, err := image.Decode(bytes.NewReader(data))
	return im, err
}

func (ic *ImageCache) storeImage(k string, m image.Image, enc func(w io.Writer, m image.Image) error) error {
	buf := new(bytes.Buffer)
	if err := enc(buf, m); err != nil {
		log.Println("can't encode image for cache:", err)
		return err
	}
	if err := ic.db.Put([]byte(k), buf.Bytes(), nil); err != nil {
		log.Println("can't store image in cache:", err)
		return err
	}
	return nil
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
