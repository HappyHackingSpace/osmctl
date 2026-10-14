package osmref

import "testing"

func TestParse(t *testing.T) {
	valid := map[string]string{
		"node/123":                             "node/123",
		"n123":                                 "node/123",
		"W42":                                  "way/42",
		"r7":                                   "relation/7",
		"relation 7":                           "relation/7",
		" way/9 ":                              "way/9",
		"https://www.openstreetmap.org/way/42": "way/42",
		"https://www.openstreetmap.org/node/5#map=19/37.9/40.2": "node/5",
	}
	for in, want := range valid {
		got, err := Parse(in)
		if err != nil {
			t.Errorf("Parse(%q): %v", in, err)
			continue
		}
		if got.String() != want {
			t.Errorf("Parse(%q) = %s, want %s", in, got, want)
		}
	}

	for _, in := range []string{"", "123", "x123", "node/", "node/abc", "https://example.org/", "changeset/1", "n999999999999", "n99999999999999999999"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q): expected error", in)
		}
	}
}
