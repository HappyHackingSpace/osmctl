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
	// The optional minus sign admits the placeholder ids of staged elements.
	short   = regexp.MustCompile(`^([nwr])(-?\d+)$`)
	long    = regexp.MustCompile(`^(node|way|relation)[/ ](-?\d+)$`)
	inURL   = regexp.MustCompile(`/(node|way|relation)/(\d+)`)
	letters = map[string]osm.Type{"n": osm.TypeNode, "w": osm.TypeWay, "r": osm.TypeRelation}
)

// Parse accepts node/123, n123, "node 123" and openstreetmap.org URLs, plus
// negative ids such as n-1 for elements staged but not yet uploaded.
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
	if err != nil || ref > maxRef || ref < -maxRef || ref == 0 {
		return 0, fmt.Errorf("invalid element id %q", digits)
	}
	return ID(t, ref)
}

// ID builds a feature id that also works for the negative placeholder ids of
// new elements. paulmach/osm v0.9.0's WayID.FeatureID does not mask negative
// ids and corrupts the type bits, so ways are derived from the node id, which
// is masked correctly, by shifting the type from node (1) to way (2).
func ID(t osm.Type, ref int64) (osm.FeatureID, error) {
	switch t {
	case osm.TypeNode:
		return osm.NodeID(ref).FeatureID(), nil
	case osm.TypeWay:
		return osm.NodeID(ref).FeatureID() + 1<<60, nil
	case osm.TypeRelation:
		return osm.RelationID(ref).FeatureID(), nil
	}
	return 0, fmt.Errorf("unsupported element type %q", t)
}

// Of returns the feature id of an element; see ID.
func Of(el osm.Element) osm.FeatureID {
	switch v := el.(type) {
	case *osm.Node:
		return osm.NodeID(v.ID).FeatureID()
	case *osm.Way:
		id, _ := ID(osm.TypeWay, int64(v.ID))
		return id
	case *osm.Relation:
		return v.ID.FeatureID()
	}
	return el.FeatureID()
}
