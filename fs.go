package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

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
				is.data = append(is.data, &fsImage{
					absPath: fp,
					modTime: cim.ModTime,
					lat:     cim.Lat,
					long:    cim.Lng,
				})
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
		is.data = append(is.data, im)
		if is.cache != nil {
			lat, long := im.LatLong()
			err := is.cache.Put(fp, &CachedImage{
				ModTime: im.ModTime(),
				Lat:     lat,
				Lng:     long,
			})
			if err != nil {
				log.Println(err)
			}
		}
		return nil
	})
}

func readImage(fp string) (Image, error) {
	f, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	x, err := exif.Decode(f)
	if err != nil {
		return nil, err
	}
	lat, lng, err := x.LatLong()
	if err != nil {
		return nil, err
	}
	return &fsImage{
		absPath: fp,
		modTime: fi.ModTime(),
		lat:     lat,
		long:    lng,
	}, nil
}

type fsImage struct {
	absPath   string
	modTime   time.Time
	lat, long float64
}

func (i *fsImage) Id() string                   { return "file://" + i.absPath }
func (i *fsImage) ModTime() time.Time           { return i.modTime }
func (i *fsImage) LatLong() (lat, long float64) { return i.lat, i.long }

func (i *fsImage) Open() io.ReadCloser {
	return ErrReadCloser(fmt.Errorf("not implemented"))
}
