package main

import "math"

// lat2merc projects latitude values (-85..85) to vertical mercator
// coordinates into the range ~(-180..180) so that locations appear
// evenly spaced out on a map using mercator projection.
func lat2merc(lat float64) float64 {
	return 180 / math.Pi * math.Log(math.Tan(math.Pi/4+lat*math.Pi/180/2))
}

// merc2lat is the inverse of lat2merc
func merc2lat(y float64) float64 {
	return 180 / math.Pi * (2*math.Atan(math.Exp(y*math.Pi/180)) - math.Pi/2)
}
