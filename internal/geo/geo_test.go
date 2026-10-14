package geo

import "testing"

func TestParsePoint(t *testing.T) {
	p, err := ParsePoint("37.91, 40.23")
	if err != nil || p.Lat != 37.91 || p.Lon != 40.23 {
		t.Fatalf("ParsePoint = %+v, %v", p, err)
	}
	for _, bad := range []string{"", "1", "1,2,3", "a,b", "91,0", "0,181"} {
		if _, err := ParsePoint(bad); err == nil {
			t.Errorf("ParsePoint(%q): expected error", bad)
		}
	}
}

func TestParseBBox(t *testing.T) {
	b, err := ParseBBox("40.2,37.9,40.3,38")
	if err != nil || b.MinLon != 40.2 || b.MaxLat != 38 {
		t.Fatalf("ParseBBox = %+v, %v", b, err)
	}
	for _, bad := range []string{"1,2,3", "40.3,37.9,40.2,38", "0,0,0,0", "-200,0,1,1"} {
		if _, err := ParseBBox(bad); err == nil {
			t.Errorf("ParseBBox(%q): expected error", bad)
		}
	}
}

func TestAround(t *testing.T) {
	b := Around(Point{Lat: 0, Lon: 0}, 111320)
	if b.MinLat > -0.999 || b.MaxLat < 0.999 || b.MaxLon < 0.999 {
		t.Errorf("Around at equator = %+v, want about ±1°", b)
	}
	b = Around(Point{Lat: 60, Lon: 10}, 1000)
	if w, h := b.MaxLon-b.MinLon, b.MaxLat-b.MinLat; w < 1.9*h {
		t.Errorf("at 60°N longitude span should be about twice latitude span: %v vs %v", w, h)
	}
}
