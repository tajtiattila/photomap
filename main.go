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
	if camsrc != "" {
		var err error
		is, err = NewCamliImageSource(camsrc)
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

	log.Fatal(http.ListenAndServe(addr, nil))
}
