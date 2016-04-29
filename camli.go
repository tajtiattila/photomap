package main

import (
	"strconv"

	"camlistore.org/pkg/client"
	"camlistore.org/pkg/schema/nodeattr"
	"camlistore.org/pkg/search"
)

type CamliImageSource struct {
	c *client.Client
}

func NewCamliImageSource(cn string) (*CamliImageSource, error) {
	c := client.New(cn)
	err := c.SetupAuth()
	if err != nil {
		return nil, err
	}
	return &CamliImageSource{c}, nil
}

func (is *CamliImageSource) GetImages() ([]Image, error) {
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
	r := make([]Image, 0, len(sr.Blobs))
	for _, srb := range sr.Blobs {
		db := sr.Describe.Meta[srb.Blob.String()]
		if db.Permanode != nil {
			pna := db.Permanode.Attr
			lat, err1 := strconv.ParseFloat(pna.Get(nodeattr.Latitude), 64)
			lng, err2 := strconv.ParseFloat(pna.Get(nodeattr.Longitude), 64)
			if err1 == nil && err2 == nil {
				r = append(r, Image{Lat: lat, Lng: lng})
			}
		}
	}
	return r, nil
}

func (is *CamliImageSource) Close() error {
	return is.c.Close()
}
