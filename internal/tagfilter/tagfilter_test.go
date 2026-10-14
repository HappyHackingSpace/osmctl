package tagfilter

import (
	"testing"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

func TestParse(t *testing.T) {
	cases := map[string]string{
		"amenity":          `["amenity"]`,
		"!name":            `[!"name"]`,
		"amenity=cafe":     `["amenity"="cafe"]`,
		"cuisine!=pizza":   `["cuisine"!="pizza"]`,
		"name~^Kafe":       `["name"~"^Kafe"]`,
		"name!~bar":        `["name"!~"bar"]`,
		`name=Say "hi"`:    `["name"="Say \"hi\""]`,
		"addr:street=Gazi": `["addr:street"="Gazi"]`,
	}
	for in, want := range cases {
		f, err := Parse(in)
		if err != nil {
			t.Errorf("Parse(%q): %v", in, err)
			continue
		}
		if f.QL() != want {
			t.Errorf("Parse(%q) = %s, want %s", in, f.QL(), want)
		}
	}
	for _, bad := range []string{"", "=cafe", "amenity=", "!amenity=cafe"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q): expected error", bad)
		}
	}
}

func TestMatch(t *testing.T) {
	tags := osm.Tags{{Key: "amenity", Value: "cafe"}, {Key: "name", Value: "Kafe Amed"}}
	cases := map[string]bool{
		"amenity":        true,
		"shop":           false,
		"!shop":          true,
		"!name":          false,
		"amenity=cafe":   true,
		"amenity=bar":    false,
		"amenity!=bar":   true,
		"cuisine!=pizza": true,
		"name~^Kafe":     true,
		"name~^Bar":      false,
		"name!~Bar":      true,
		"website!~https": true,
	}
	for in, want := range cases {
		f, err := Parse(in)
		if err != nil {
			t.Fatal(err)
		}
		if got := f.Match(tags); got != want {
			t.Errorf("%s.Match = %v, want %v", in, got, want)
		}
	}
	if _, err := Parse("name~("); err == nil {
		t.Error("invalid regex should fail to parse")
	}
}
