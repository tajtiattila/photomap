package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tajtiattila/photomap/imagecache"
)

func NewTileHandler(f func(x, y, zoom int) []byte) http.Handler {
	starttime := time.Now()
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		s := req.URL.Path
		if len(s) == 0 || s[0] != '/' {
			http.Error(w, "invalid tile path", http.StatusBadRequest)
			return
		}
		parts := strings.Split(s[1:], "_")
		if len(parts) != 3 {
			http.Error(w, "invalid tile path", http.StatusBadRequest)
			return
		}
		x, err := strconv.Atoi(parts[0])
		if err != nil {
			http.Error(w, "x invalid: "+err.Error(), http.StatusBadRequest)
			return
		}
		y, err := strconv.Atoi(parts[1])
		if err != nil {
			http.Error(w, "y invalid: "+err.Error(), http.StatusBadRequest)
			return
		}
		zoom, err := strconv.Atoi(parts[2])
		if err != nil {
			http.Error(w, "zoom invalid: "+err.Error(), http.StatusBadRequest)
			return
		}
		xmask := (1 << uint(zoom)) - 1
		x = x & xmask
		data := f(x, y, zoom)
		http.ServeContent(w, req, "tile.png", starttime, bytes.NewReader(data))
	})
}

func NewViewportPlaceHandler(tm *TileMap) http.Handler {
	starttime := time.Now()
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		v := req.URL.Query()
		var eh errh
		la0, lo0 := eh.parseFloat(v.Get("la0")), eh.parseFloat(v.Get("lo0"))
		la1, lo1 := eh.parseFloat(v.Get("la1")), eh.parseFloat(v.Get("lo1"))
		zoom := eh.atoi(v.Get("zoom"))
		if eh.handleError(w, "bounds/zoom invalid") {
			return
		}
		places, dist := tm.PhotoPlaces(la0, lo0, la1, lo1, zoom)
		coords := make([]json.Number, 0, len(places)*2)
		for _, p := range places {
			coords = append(coords,
				json.Number(fmt.Sprintf("%.6f", p.Lat)),
				json.Number(fmt.Sprintf("%.6f", p.Long)))
		}
		type viewPortResponse struct {
			Radius float64       `json:"radius"`
			Coords []json.Number `json:"coords"`
		}
		serveJson(w, req, viewPortResponse{dist, coords}, starttime)
	})
}

func NewGalleryHandler(tm *TileMap) http.Handler {
	starttime := time.Now()
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		v := req.URL.Query()
		var eh errh
		lat, long := eh.parseFloat(v.Get("la")), eh.parseFloat(v.Get("lo"))
		zoom := eh.atoi(v.Get("zoom"))
		if eh.handleError(w, "loc/zoom invalid") {
			return
		}
		res := tm.Gallery(lat, long, zoom)
		if len(res) == 0 {
			http.NotFound(w, req)
			return
		}
		serveJson(w, req, res, starttime)
	})
}

func NewThumbnailHandler(ic *imagecache.ImageCache) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		key := req.URL.Path
		if len(key) == 0 || key[0] != '/' {
			http.Error(w, "invalid thumb path", http.StatusBadRequest)
			return
		}
		r, mt, err := ic.Thumbnail(key[1:])
		if err != nil {
			log.Println(err)
			http.NotFound(w, req)
			return
		}
		http.ServeContent(w, req, "thumb.jpeg", mt, r)
	})
}

type errh struct {
	err error
}

func (e *errh) atoi(s string) (i int) {
	if e.err != nil {
		return
	}
	i, e.err = strconv.Atoi(s)
	return i
}

func (e *errh) parseFloat(s string) (v float64) {
	if e.err != nil {
		return
	}
	v, e.err = strconv.ParseFloat(s, 64)
	return v
}

func (e *errh) handleError(w http.ResponseWriter, errmsg string) bool {
	if e.err != nil {
		http.Error(w, fmt.Sprintf("%s: %v", errmsg, e.err), http.StatusBadRequest)
		return true
	}
	return false
}

func serveJson(w http.ResponseWriter, req *http.Request, data interface{}, mt time.Time) {
	raw, err := json.Marshal(data)
	if err != nil {
		log.Println(err)
		http.Error(w, fmt.Sprintf("can't marshal %v: %v", data, err), http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, req, "data.json", mt, bytes.NewReader(raw))
}
