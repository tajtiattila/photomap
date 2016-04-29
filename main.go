package main

import (
	"fmt"
	"log"
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
