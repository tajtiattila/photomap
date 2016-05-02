package main

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/syndtr/goleveldb/leveldb"
)

type leveldbCache struct {
	db *leveldb.DB
}

func NewLevelDbCache(path string) (ImageInfoCache, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}
	return &leveldbCache{db}, nil
}

func (c *leveldbCache) Get(id string) (*CachedImage, error) {
	raw, err := c.db.Get([]byte(id), nil)
	if err == leveldb.ErrNotFound {
		err = os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	im := new(CachedImage)
	if err := json.Unmarshal(raw, im); err != nil {
		return nil, err
	}
	return im, nil
}

func (c *leveldbCache) Put(id string, im *CachedImage) error {
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(*im)
	if err != nil {
		return err
	}
	return c.db.Put([]byte(id), buf.Bytes(), nil)
}

func (c *leveldbCache) Close() error {
	return c.db.Close()
}
