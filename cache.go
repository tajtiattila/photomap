package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/tajtiattila/basedir"
)

type ImageCache struct {
	src ImageSource
	db  *leveldb.DB

	m map[string]ImageInfo
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
		src: src,
		db:  db,
		m:   make(map[string]ImageInfo),
	}
	return ic, ic.init()
}

func (ic *ImageCache) GetImages() map[string]ImageInfo {
	return ic.m
}

func (ic *ImageCache) Close() error {
	ic.src.Close()
	return ic.db.Close()
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
