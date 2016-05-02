package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/rwcarlsen/goexif/exif"
)

type FileSystemImageSource struct {
	cache ImageInfoCache

	data []Image
}

func NewFileSystemImageSource(root string, cache ImageInfoCache) (*FileSystemImageSource, error) {
	is := &FileSystemImageSource{cache: cache}
	err := is.prepare(root)
	if err != nil {
		return nil, err
	}
	return is, nil
}

func (is *FileSystemImageSource) GetImages() ([]Image, error) {
	return is.data, nil
}

func (is *FileSystemImageSource) Close() error {
	return nil
}

func (is *FileSystemImageSource) prepare(root string) error {
	absroot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	return filepath.Walk(absroot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Println(err)
			return nil
		}
		if info.IsDir() {
			return nil
		}
		fp := filepath.ToSlash(path)
		if is.cache != nil {
			cim, err := is.cache.Get(fp)
			if err == nil && !info.ModTime().After(cim.ModTime) {
				is.data = append(is.data, Image{Lat: cim.Lat, Lng: cim.Lng})
				return nil // cache valid
			}
		}
		im, err := readImage(fp)
		if err != nil {
			if _, ok := err.(exif.TagNotPresentError); !ok {
				log.Println(err)
			}
			return nil // not an image, or no GPS info
		}
		is.data = append(is.data, *im)
		if is.cache != nil {
			err := is.cache.Put(fp, &CachedImage{
				ModTime: info.ModTime(),
				Lat:     im.Lat,
				Lng:     im.Lng,
			})
			if err != nil {
				log.Println(err)
			}
		}
		return nil
	})
}

func readImage(fp string) (*Image, error) {
	f, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	x, err := exif.Decode(f)
	if err != nil {
		return nil, err
	}
	lat, lng, err := x.LatLong()
	if err != nil {
		return nil, err
	}
	return &Image{Lat: lat, Lng: lng}, nil
}
