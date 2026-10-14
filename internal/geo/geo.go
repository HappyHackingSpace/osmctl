// Package geo parses the coordinates and bounding boxes users type.
package geo

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/paulmach/osm"
)

type Point struct{ Lat, Lon float64 }

func (p Point) String() string { return fmt.Sprintf("%g,%g", p.Lat, p.Lon) }

// ParsePoint reads "lat,lon", the order used by OSM and by most map URLs.
func ParsePoint(s string) (Point, error) {
	v, err := floats(s, 2)
	if err != nil {
		return Point{}, fmt.Errorf("invalid point %q (expected lat,lon): %w", s, err)
	}
	p := Point{Lat: v[0], Lon: v[1]}
	if p.Lat < -90 || p.Lat > 90 || p.Lon < -180 || p.Lon > 180 {
		return Point{}, fmt.Errorf("point %q out of range (lat -90..90, lon -180..180)", s)
	}
	return p, nil
}

// ParseBBox reads "minlon,minlat,maxlon,maxlat", the order the OSM API uses.
func ParseBBox(s string) (*osm.Bounds, error) {
	v, err := floats(s, 4)
	if err != nil {
		return nil, fmt.Errorf("invalid bbox %q (expected minlon,minlat,maxlon,maxlat): %w", s, err)
	}
	b := &osm.Bounds{MinLon: v[0], MinLat: v[1], MaxLon: v[2], MaxLat: v[3]}
	if b.MinLon >= b.MaxLon || b.MinLat >= b.MaxLat {
		return nil, fmt.Errorf("invalid bbox %q: min must be below max", s)
	}
	if b.MinLat < -90 || b.MaxLat > 90 || b.MinLon < -180 || b.MaxLon > 180 {
		return nil, fmt.Errorf("bbox %q out of range", s)
	}
	return b, nil
}

func floats(s string, n int) ([]float64, error) {
	parts := strings.Split(s, ",")
	if len(parts) != n {
		return nil, fmt.Errorf("want %d comma-separated numbers, got %d", n, len(parts))
	}
	out := make([]float64, n)
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, err
		}
		out[i] = f
	}
	return out, nil
}

// Around returns the box enclosing a circle of radius meters, for APIs that
// only take a bbox.
func Around(p Point, meters float64) *osm.Bounds {
	const metersPerDegree = 111320.0
	dLat := meters / metersPerDegree
	dLon := meters / (metersPerDegree * math.Max(math.Cos(p.Lat*math.Pi/180), 0.01))
	return &osm.Bounds{
		MinLat: math.Max(p.Lat-dLat, -90), MaxLat: math.Min(p.Lat+dLat, 90),
		MinLon: math.Max(p.Lon-dLon, -180), MaxLon: math.Min(p.Lon+dLon, 180),
	}
}
