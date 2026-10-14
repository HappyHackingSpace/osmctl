// Package gpx reads GPS tracks and simplifies them into road geometry.
package gpx

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
)

type Point struct{ Lat, Lon float64 }

type Track struct {
	Name   string
	Points []Point
}

type xPoint struct {
	Lat float64 `xml:"lat,attr"`
	Lon float64 `xml:"lon,attr"`
}

type xGPX struct {
	Tracks []struct {
		Name     string `xml:"name"`
		Segments []struct {
			Points []xPoint `xml:"trkpt"`
		} `xml:"trkseg"`
	} `xml:"trk"`
	Routes []struct {
		Name   string   `xml:"name"`
		Points []xPoint `xml:"rtept"`
	} `xml:"rte"`
}

// Read returns every track and route in the file. A track's segments are
// joined: devices split segments on signal loss, not where the road ends.
func Read(r io.Reader) ([]Track, error) {
	var x xGPX
	if err := xml.NewDecoder(r).Decode(&x); err != nil {
		return nil, fmt.Errorf("read gpx: %w", err)
	}
	var out []Track
	add := func(name string, pts []xPoint) {
		t := Track{Name: name}
		for _, p := range pts {
			t.Points = append(t.Points, Point(p))
		}
		if len(t.Points) > 0 {
			out = append(out, t)
		}
	}
	for _, t := range x.Tracks {
		var pts []xPoint
		for _, s := range t.Segments {
			pts = append(pts, s.Points...)
		}
		add(t.Name, pts)
	}
	for _, r := range x.Routes {
		add(r.Name, r.Points)
	}
	return out, nil
}

// Simplify drops points that lie within tolerance metres of the line through
// their neighbours (Douglas–Peucker), keeping the shape with far fewer nodes.
func Simplify(pts []Point, tolerance float64) []Point {
	if len(pts) < 3 {
		return pts
	}
	keep := make([]bool, len(pts))
	keep[0], keep[len(pts)-1] = true, true
	var walk func(first, last int)
	walk = func(first, last int) {
		maxD, idx := 0.0, -1
		for i := first + 1; i < last; i++ {
			if d := offset(pts[i], pts[first], pts[last]); d > maxD {
				maxD, idx = d, i
			}
		}
		if idx >= 0 && maxD > tolerance {
			keep[idx] = true
			walk(first, idx)
			walk(idx, last)
		}
	}
	walk(0, len(pts)-1)
	out := make([]Point, 0, len(pts))
	for i, p := range pts {
		if keep[i] {
			out = append(out, p)
		}
	}
	return out
}

// offset is the distance in metres from p to the segment a-b, in a local plane.
func offset(p, a, b Point) float64 {
	const r = 6371008.8
	k := math.Cos(a.Lat * math.Pi / 180)
	xy := func(q Point) (float64, float64) {
		return (q.Lon - a.Lon) * math.Pi / 180 * r * k, (q.Lat - a.Lat) * math.Pi / 180 * r
	}
	px, py := xy(p)
	bx, by := xy(b)
	t := 0.0
	if l := bx*bx + by*by; l > 0 {
		t = max(0, min(1, (px*bx+py*by)/l))
	}
	return math.Hypot(px-t*bx, py-t*by)
}
