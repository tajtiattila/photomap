package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/tajtiattila/basedir"
)

func main() {
	var addr, camsrc, dir string
	flag.StringVar(&addr, "addr", ":6677", "listen address")
	flag.StringVar(&camsrc, "camli", "", "use camlistore server as source")
	flag.StringVar(&dir, "dir", "", "use directory as image source")
	flag.Parse()

	var is ImageSource
	if camsrc != "" {
		var err error
		is, err = NewCamliImageSource(camsrc)
		if err != nil {
			log.Fatal(err)
		}
	}
	if dir != "" {
		cache := getImageInfoCache()
		if cache != nil {
			defer cache.Close()
		}
		var err error
		is, err = NewFileSystemImageSource(dir, cache)
		if err != nil {
			log.Fatal(err)
		}
	}

	if is == nil {
		log.Fatal("no image source specified")
	}

	vim, err := is.GetImages()
	if err != nil {
		log.Fatal(err)
	}

	ist := time.Now()

	p, err := filepath.Abs("res")
	if err != nil {
		panic(err)
	}

	http.Handle("/", http.FileServer(http.Dir(p)))

	http.HandleFunc("/photos.json", func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(vim)
		if err != nil {
			log.Println(err)
		}
		http.ServeContent(w, r, "photos.json", ist, bytes.NewReader(buf.Bytes()))
	})

	log.Println("Listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func getImageInfoCache() ImageInfoCache {
	cachedir, err := basedir.Cache.EnsureDir("PhotoMap", 0700)
	if err != nil {
		log.Println(err)
		return nil
	}
	cache, err := NewLevelDbCache(filepath.Join(cachedir, "imagecache.leveldb"))
	if err != nil {
		log.Println(err)
		return nil
	}
	return cache
}
