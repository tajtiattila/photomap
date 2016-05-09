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

	inf map[string]camliInfo
}

type camliInfo struct {
	source.ImageInfo
	content blob.Ref
}

const camliprefix = "camli:"

func NewCamliImageSource(cn string) (*CamliImageSource, error) {
	c := client.New(cn)
	err := c.SetupAuth()
	if err != nil {
		return nil, err
	}
	return &CamliImageSource{c, make(map[string]camliInfo)}, nil
}

func (is *CamliImageSource) ModTimes() (map[string]time.Time, error) {
	sq := search.SearchQuery{
		Expression: "has:location",
		Describe: &search.DescribeRequest{
			Rules: []*search.DescribeRule{
				{
					Attrs: []string{nodeattr.Latitude, nodeattr.Longitude},
				},
			},
		},
	}
	sr, err := is.c.Query(&sq)
	if err != nil {
		return nil, err
	}
	r := make(map[string]time.Time)
	for _, srb := range sr.Blobs {
		db := sr.Describe.Meta[srb.Blob.String()]
		contentRef, ok := db.ContentRef()
		if !ok || db.Permanode == nil {
			continue
		}
		pna := db.Permanode.Attr
		lat, err1 := strconv.ParseFloat(pna.Get(nodeattr.Latitude), 64)
		lng, err2 := strconv.ParseFloat(pna.Get(nodeattr.Longitude), 64)
		if err1 != nil && err2 != nil {
			continue
		}
		id := srb.Blob.String()
		r[camliprefix+id] = db.Permanode.ModTime
		is.inf[id] = camliInfo{
			ImageInfo: source.ImageInfo{
				ModTime: db.Permanode.ModTime,
				Lat:     lat,
				Long:    lng,
			},
			content: contentRef,
		}
	}
	return r, nil
}

func (is *CamliImageSource) Info(id string) (ii source.ImageInfo, err error) {
	ci, ok := is.inf[strings.TrimPrefix(id, camliprefix)]
	if !ok {
		return ii, os.ErrNotExist
	}
	return ci.ImageInfo, nil
}

func (is *CamliImageSource) Open(id string) (io.ReadCloser, error) {
	ci, ok := is.inf[strings.TrimPrefix(id, camliprefix)]
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

func (is *CamliImageSource) Close() error {
	return is.c.Close()
}
