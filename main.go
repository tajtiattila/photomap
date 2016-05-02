package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

func main() {
	var addr, camsrc string
	flag.StringVar(&addr, "addr", ":6677", "listen address")
	flag.StringVar(&camsrc, "camli", "", "use camlistore server as source")
	flag.Parse()

	var is ImageSource
	var err error
	if camsrc != "" {
		is, err = NewCamliImageSource(camsrc)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		if flag.NArg() == 0 {
			log.Fatal("need path argument(s)")
		}
		var err error
		is, err = NewFileSystemImageSource(flag.Args()...)
		if err != nil {
			log.Fatal(err)
		}
	}

	if is == nil {
		log.Fatal("no image source specified")
	}

	ic, err := NewImageCache(is)
	if err != nil {
		log.Fatal(err)
	}
	defer ic.Close()

	type img struct {
		Lat  float64 `json:"lat"`
		Long float64 `json:"lng"`
	}
	vim := make([]img, 0, len(ic.GetImages()))
	for _, ii := range ic.GetImages() {
		vim = append(vim, img{ii.Lat, ii.Long})
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
