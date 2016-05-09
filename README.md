photomap
========

Photomap shows your photos on a map using the Google Maps JavaScript API.

![Screenshot](/misc/screenshot.png)

If you have [Go](http://golang.org) photomap can be installed with:

    go get github.com/tajtiattila/photomap

Then set the environment variable `GOOGLEMAPS_APIKEY` to your google maps api key,
and start photomap with path(s) to your geotagged photos.

Goals
-----

Short term

- Add gallery
- Camlistore support

Future

- Ability to fix photo locations
- Track logs as location source
- Ability to fix photo times (incorrect camera time setting wrt tracks)
