package filter

import (
	"math"

	"github.com/ardean/earthquake-notifier/config"
	"github.com/ardean/earthquake-notifier/fetcher"
)

const earthRadiusKm = 6371.0

func WithinWatchArea(event fetcher.Event, watch config.WatchArea) bool {
	if !watch.Configured() {
		return true
	}
	dist := HaversineKm(watch.Latitude, watch.Longitude, event.Latitude, event.Longitude)
	return dist <= watch.RadiusKm
}

func HaversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := toRadians(lat2 - lat1)
	dLon := toRadians(lon2 - lon1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRadians(lat1))*math.Cos(toRadians(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
}

func toRadians(deg float64) float64 {
	return deg * math.Pi / 180
}
