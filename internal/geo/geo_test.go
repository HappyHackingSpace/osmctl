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
