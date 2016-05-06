package fs

import (
	"bytes"
	"image"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/tajtiattila/photomap/source"
)

func init() {
	source.Register("filesystem", func(paths string) (source.ImageSource, error) {
		return NewFileSystemImageSource(strings.Split(paths, string(os.PathSeparator))...)
	})
}

type FileSystemImageSource struct {
	modTimes map[string]time.Time
}

const fsprefix = "file://"

func NewFileSystemImageSource(paths ...string) (*FileSystemImageSource, error) {
	is := &FileSystemImageSource{
		modTimes: make(map[string]time.Time),
	}
	for _, p := range paths {
		err := is.prepare(p)
		if err != nil {
			return nil, err
		}
	}
	return is, nil
}

func (is *FileSystemImageSource) ModTimes() (map[string]time.Time, error) {
	return is.modTimes, nil
}

func (is *FileSystemImageSource) Info(id string) (ii source.ImageInfo, err error) {
	f, err := is.open(id)
	if err != nil {
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return source.ImageInfo{}, err
	}
	ii.ModTime = fi.ModTime()

	buf := new(bytes.Buffer)
	tr := io.TeeReader(f, buf)
	cfg, _, err := image.DecodeConfig(tr)
	if err != nil {
		return source.ImageInfo{}, err
	}
	ii.Width, ii.Height = cfg.Width, cfg.Height

	x, err := exif.Decode(io.MultiReader(buf, f))
	if err != nil {
		return source.ImageInfo{}, err
	}
	ii.Lat, ii.Long, err = x.LatLong()
	return ii, nil
}

func (is *FileSystemImageSource) Open(id string) (io.ReadCloser, error) {
	return is.open(id)
}

func (is *FileSystemImageSource) open(id string) (*os.File, error) {
	if !strings.HasPrefix(id, fsprefix) {
		return nil, os.ErrNotExist
	}
	return os.Open(strings.TrimPrefix(id, fsprefix))
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
		is.modTimes[fsprefix+fp] = info.ModTime()
		return nil
	})
}
