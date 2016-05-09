package fs

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

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

func (is *FileSystemImageSource) ModTimes() map[string]time.Time {
	return is.modTimes
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

	return source.InfoFromReader(fi.ModTime(), f)
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
