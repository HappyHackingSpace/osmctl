// Package osmref parses the many ways people write an OSM element reference.
package osmref

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/paulmach/osm"
)

var (
	short   = regexp.MustCompile(`^([nwr])(\d+)$`)
	long    = regexp.MustCompile(`^(node|way|relation)[/ ](\d+)$`)
	inURL   = regexp.MustCompile(`/(node|way|relation)/(\d+)`)
	letters = map[string]osm.Type{"n": osm.TypeNode, "w": osm.TypeWay, "r": osm.TypeRelation}
)

// Parse accepts node/123, n123, "node 123" and openstreetmap.org URLs.
func Parse(s string) (osm.FeatureID, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if m := short.FindStringSubmatch(s); m != nil {
		return build(letters[m[1]], m[2])
	}
	if m := long.FindStringSubmatch(s); m != nil {
		return build(osm.Type(m[1]), m[2])
	}
	if u, err := url.Parse(s); err == nil && u.Host != "" {
		if m := inURL.FindStringSubmatch(u.Path); m != nil {
			return build(osm.Type(m[1]), m[2])
		}
	}
	return 0, fmt.Errorf("invalid element %q (expected node/123, n123 or an openstreetmap.org URL)", s)
}

// maxRef is the largest id paulmach/osm can pack into a FeatureID (40 signed
// bits); larger ids would silently wrap around.
const maxRef = 1<<39 - 1

func build(t osm.Type, digits string) (osm.FeatureID, error) {
	ref, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || ref > maxRef {
		return 0, fmt.Errorf("invalid element id %q", digits)
	}
	return t.FeatureID(ref)
}
