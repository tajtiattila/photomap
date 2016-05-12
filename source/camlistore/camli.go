package camlistore

import (
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"camlistore.org/pkg/blob"
	"camlistore.org/pkg/client"
	"camlistore.org/pkg/schema"
	"camlistore.org/pkg/schema/nodeattr"
	"camlistore.org/pkg/search"

	"github.com/tajtiattila/photomap/source"
)

func init() {
	source.Register("camlistore", func(cn string) (source.ImageSource, error) {
		return NewCamliImageSource(cn)
	})
}

type CamliImageSource struct {
	c *client.Client

	m map[string]time.Time

	cam map[string]camliInfo
}

type camliInfo struct {
	mt      time.Time
	ploc    *latlng // location from permanode attrs, if any
	content blob.Ref
}

type latlng struct {
	lat, lng float64
}

const camliprefix = "camli:"

func NewCamliImageSource(cn string) (*CamliImageSource, error) {
	c := client.New(cn)
	err := c.SetupAuth()
	if err != nil {
		return nil, err
	}
	is := &CamliImageSource{c: c}
	return is, is.init()
}

func (is *CamliImageSource) Close() error {
	return is.c.Close()
}

func (is *CamliImageSource) ModTimes() map[string]time.Time {
	return is.m
}

func (is *CamliImageSource) Info(id string) (ii source.ImageInfo, err error) {
	sref := strings.TrimPrefix(id, camliprefix)
	rc, err := is.open(sref)
	if err != nil {
		return source.ImageInfo{}, err
	}
	defer rc.Close()

	ci := is.cam[sref]

	ii, err = source.InfoFromReader(ci.mt, rc)

	ok := err == nil || (source.IsNoLoc(err) && ci.ploc != nil)
	if !ok {
		return source.ImageInfo{}, err
	}

	if ci.ploc != nil {
		// set/overwrite lat/long with those in permanode
		ii.Lat, ii.Long = ci.ploc.lat, ci.ploc.lng
	}
	return ii, nil
}

func (is *CamliImageSource) Open(id string) (io.ReadCloser, error) {
	return is.open(strings.TrimPrefix(id, camliprefix))
}

func (is *CamliImageSource) open(id string) (io.ReadCloser, error) {
	ci, ok := is.cam[strings.TrimPrefix(id, camliprefix)]
	if !ok {
		log.Printf("missing %q", id)
		return nil, os.ErrNotExist
	}
	rc, err := schema.NewFileReader(is.c, ci.content)
	if err != nil {
		log.Printf("camlistore: fetch failed for %q (%q)", id, ci.content)
		return nil, os.ErrNotExist
	}
	return rc, nil
}

func (is *CamliImageSource) init() error {
	start := true
	var cont string
	for start || cont != "" {
		start = false
		sq := search.SearchQuery{
			Expression: "has:location",
			Describe: &search.DescribeRequest{
				Rules: []*search.DescribeRule{
					{
						Attrs: []string{nodeattr.Latitude, nodeattr.Longitude},
					},
				},
			},
			Continue: cont,
		}
		sr, err := is.c.Query(&sq)
		if err != nil {
			return err
		}
		cont = sr.Continue
		if is.m == nil {
			is.m = make(map[string]time.Time)
		}
		if is.cam == nil {
			is.cam = make(map[string]camliInfo)
		}
		for _, srb := range sr.Blobs {
			id := srb.Blob.String()
			db := sr.Describe.Meta[id]
			contentRef, ok := db.ContentRef()
			if !ok || db.Permanode == nil {
				continue
			}
			pna := db.Permanode.Attr
			lat, err1 := strconv.ParseFloat(pna.Get(nodeattr.Latitude), 64)
			lng, err2 := strconv.ParseFloat(pna.Get(nodeattr.Longitude), 64)
			var ll *latlng
			if err1 == nil && err2 == nil {
				ll = &latlng{lat, lng}
			}
			is.m[camliprefix+id] = db.Permanode.ModTime
			is.cam[id] = camliInfo{
				mt:      db.Permanode.ModTime,
				ploc:    ll,
				content: contentRef,
			}
		}
	}
	return nil
}
