package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tajtiattila/photomap/imagecache"
	"github.com/tajtiattila/photomap/source"
	_ "github.com/tajtiattila/photomap/source/camlistore"
	_ "github.com/tajtiattila/photomap/source/filesystem"
)

func main() {
	var addr, camsrc string
	flag.StringVar(&addr, "addr", ":6677", "listen address")
	flag.StringVar(&camsrc, "camli", "", "use camlistore server as source")
	flag.Parse()

	gmapsapikey := os.Getenv("GOOGLEMAPS_APIKEY")
	if gmapsapikey == "" {
		log.Fatal("GOOGLEMAPS_APIKEY environment variable unset")
	}

	var is source.ImageSource
	var err error
	if camsrc != "" {
		is, err = source.Open("camlistore", camsrc)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		if flag.NArg() == 0 {
			log.Fatal("need path argument(s)")
		}
		var err error
		is, err = source.Open("filesystem", strings.Join(flag.Args(), string(os.PathSeparator)))
		if err != nil {
			log.Fatal(err)
		}
	}

	if is == nil {
		log.Fatal("no image source specified")
	}

	ic, err := imagecache.New(is)
	if err != nil {
		log.Fatal(err)
	}
	defer ic.Close()

	tm := NewTileMap(ic)

	type img struct {
		Lat  float64 `json:"lat"`
		Long float64 `json:"lng"`
	}
	vim := make([]img, 0, len(ic.Images()))
	for _, ii := range ic.Images() {
		vim = append(vim, img{ii.Lat, ii.Long})
	}

	ist := time.Now()

	p, err := filepath.Abs("res")
	if err != nil {
		panic(err)
	}

	templateData := struct {
		GoogleMapsApiKey string
	}{
		gmapsapikey,
	}
	http.Handle("/", http.FileServer(&templateDir{p, templateData}))

	http.HandleFunc("/photos.json", func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(vim)
		if err != nil {
			log.Println(err)
		}
		http.ServeContent(w, r, "photos.json", ist, bytes.NewReader(buf.Bytes()))
	})

	http.Handle("/tiles/", tm)

	log.Println("Listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// templateDir is like http.Dir but applies
// the template arguments to html files.
type templateDir struct {
	root string
	data interface{} // template data
}

func (td *templateDir) Open(name string) (http.File, error) {
	f, err := http.Dir(td.root).Open(name)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(name, ".html") {
		return f, nil
	}
	src, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	t, err := template.New(name).Parse(string(src))
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	if err := t.Execute(buf, td.data); err != nil {
		return nil, err
	}
	return &templateFile{f, bytes.NewReader(buf.Bytes())}, nil
}

type templateFile struct {
	http.File
	r *bytes.Reader
}

func (f *templateFile) Read(p []byte) (n int, err error) {
	return f.r.Read(p)
}

func (f *templateFile) Seek(offset int64, whence int) (int64, error) {
	return f.r.Seek(offset, whence)
}
