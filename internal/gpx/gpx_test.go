package gpx

import (
	"strings"
	"testing"
)

const sample = `<?xml version="1.0"?>
<gpx version="1.1" creator="test" xmlns="http://www.topografix.com/GPX/1/1">
  <trk><name>morning</name>
    <trkseg><trkpt lat="38.0000" lon="40.0000"/><trkpt lat="38.0001" lon="40.0000"/></trkseg>
    <trkseg><trkpt lat="38.0002" lon="40.0000"/></trkseg>
  </trk>
  <rte><name>planned</name><rtept lat="1" lon="2"/><rtept lat="1.5" lon="2.5"/></rte>
</gpx>`

func TestRead(t *testing.T) {
	tracks, err := Read(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 2 || tracks[0].Name != "morning" || len(tracks[0].Points) != 3 || tracks[1].Name != "planned" {
		t.Fatalf("tracks = %+v", tracks)
	}
	if _, err := Read(strings.NewReader("not xml")); err == nil {
		t.Error("expected an error for invalid input")
	}
}

func TestSimplify(t *testing.T) {
	// A straight line with GPS jitter under 2 m, and one real 50 m corner.
	var pts []Point
	for i := 0; i <= 100; i++ {
		jitter := 0.000015 * float64(i%2) // about 1.7 m
		pts = append(pts, Point{38 + jitter, 40 + float64(i)*0.00001})
	}
	pts = append(pts, Point{38.00045, 40.001})
	got := Simplify(pts, 3)
	if len(got) != 3 {
		t.Fatalf("kept %d points, want start, corner and end: %+v", len(got), got)
	}
	if got[0] != pts[0] || got[2] != pts[len(pts)-1] {
		t.Error("simplify must keep both ends")
	}
	if fine := Simplify(pts, 0.5); len(fine) <= 3 {
		t.Errorf("a tighter tolerance should keep more points, kept %d", len(fine))
	}
	if two := Simplify(pts[:2], 3); len(two) != 2 {
		t.Error("two points stay two points")
	}
}
