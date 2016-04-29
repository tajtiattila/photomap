package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"camlistore.org/pkg/client"
)

func main() {
	is, err := NewCamliImageSource("vulcan")
	if err != nil {
		log.Fatal(err)
	}
	vim, err := is.GetImages()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(vim)
}

func main0() {

	c := client.New("vulcan")
	err := c.SetupAuth()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	cr, err := clientRoot(c)
	if err != nil {
		log.Fatal(err)
	}

	blob := "sha1-f4711e11a0b7dc37ebf9c4faad9caa6b180367f3"
	req, err := http.NewRequest("GET", cr+"ui/thumbnail/"+blob+"?mw=16&mh=16", nil)
	if err != nil {
		log.Fatal(err)
	}
	req.SetBasicAuth("ata", "mafo6fe1cho4ti")

	r, err := c.HTTPClient().Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Body.Close()
	io.Copy(os.Stdout, r.Body)
}

func clientRoot(c *client.Client) (string, error) {
	br, err := c.BlobRoot()
	if err != nil {
		return "", err
	}
	for len(br) > 0 && br[len(br)-1] == '/' {
		br = br[:len(br)-1]
	}
	i := strings.LastIndexByte(br, '/')
	if i != -1 {
		return br[:i+1], nil
	}
	return br + "/", nil
}
