package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"log"
	"path/filepath"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/tajtiattila/basedir"
)

type ImageCache struct {
	src ImageSource
	db  *leveldb.DB

	m map[string]ImageInfo

	pimtx sync.Mutex
	pigen map[string]*genPhotoIcon
}

func NewImageCache(src ImageSource) (*ImageCache, error) {
	cachedir, err := basedir.Cache.EnsureDir("PhotoMap", 0700)
	if err != nil {
		return nil, err
	}
	db, err := leveldb.OpenFile(filepath.Join(cachedir, "imagecache.leveldb"), nil)
	if err != nil {
		return nil, err
	}
	ic := &ImageCache{
		src:   src,
		db:    db,
		m:     make(map[string]ImageInfo),
		pigen: make(map[string]*genPhotoIcon),
	}
	return ic, ic.init()
}

func (ic *ImageCache) Images() map[string]ImageInfo {
	return ic.m
}

func (ic *ImageCache) Close() error {
	ic.src.Close()
	return ic.db.Close()
}

func (ic *ImageCache) PhotoIcon(id string) (image.Image, error) {
	ic.pimtx.Lock()
	pg, ok := ic.pigen[id]
	if !ok {
		pg = new(genPhotoIcon)
		ic.pigen[id] = pg
	}
	ic.pimtx.Unlock()

	pg.once.Do(func() {
		var err error
		pg.im, err = ic.genPhotoIcon(pg, id)
		if err != nil {
			log.Println(err)
		}
	})

	if pg.im == nil {
		return nil, fmt.Errorf("missing thumb for %s", id)
	}
	return pg.im, nil
}

func (ic *ImageCache) genPhotoIcon(pg *genPhotoIcon, id string) (image.Image, error) {
	k := []byte("photoicon|" + id)
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

	log.Println("thumbing", id)

	// todo: sync.Once
	rc, err := ic.src.Open(id)
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

	kbuf := append(make([]byte, 200), "imageinfo|"...)
	vid := make([]string, 0, len(mmt))
	for id, mt := range mmt {
		data, err := ic.db.Get(append(kbuf, id...), nil)
		if err == leveldb.ErrNotFound {
			vid = append(vid, id)
			continue
		}
		if err != nil {
			return err
		}
		var ce cacheEntry
		if err = json.Unmarshal(data, &ce); err != nil {
			return err
		}
		if ce.ModTime.Before(mt) {
			vid = append(vid, id)
			continue
		}
		if ce.IsErr {
			continue
		}
		ic.m[id] = ce.ImageInfo
	}

	var dbuf bytes.Buffer
	for _, id := range vid {
		ii, err := ic.src.Info(id)
		var ce cacheEntry
		if err != nil {
			ce.IsErr = true
		} else {
			ce.ImageInfo = ii
			ic.m[id] = ii
		}
		dbuf.Reset()
		if err := json.NewEncoder(&dbuf).Encode(ii); err != nil {
			return err
		}
		if err := ic.db.Put(append(kbuf, id...), dbuf.Bytes(), nil); err != nil {
			return err
		}
	}
	return nil
}

type cacheEntry struct {
	ImageInfo
	IsErr bool
}

type genPhotoIcon struct {
	once sync.Once
	im   image.Image
}
