package overpass

import (
	"fmt"
	"strings"

	"github.com/paulmach/osm"

	"github.com/happyhackingspace/osmctl/internal/tagfilter"
)

// Scope limits where Find looks. Exactly one field should be set.
type Scope struct {
	Bounds *osm.Bounds
	Around *Around
	// AreaID is an Overpass area id, derived from an OSM way or relation.
	AreaID int64
}

type Around struct {
	Lat, Lon float64
	Radius   float64
}

// AreaIDFor converts an OSM way or relation into the id Overpass uses for the area it encloses.
func AreaIDFor(id osm.FeatureID) (int64, error) {
	switch id.Type() {
	case osm.TypeWay:
		return 2400000000 + id.Ref(), nil
	case osm.TypeRelation:
		return 3600000000 + id.Ref(), nil
	}
	return 0, fmt.Errorf("%s does not enclose an area", id)
}

type FindQuery struct {
	Types   []osm.Type
	Filters []tagfilter.Filter
	Scope   Scope
	Limit   int
	Timeout int
}

// Build renders the query. The `>; out skel qt` tail fetches the nodes of
// matched ways and relations, so every output format, GeoJSON included, can
// draw them.
func (q FindQuery) Build() (string, error) {
	if len(q.Filters) == 0 {
		return "", fmt.Errorf("at least one tag filter is required")
	}
	var scope string
	var pre string
	switch {
	case q.Scope.Bounds != nil:
		b := q.Scope.Bounds
		scope = fmt.Sprintf("(%g,%g,%g,%g)", b.MinLat, b.MinLon, b.MaxLat, b.MaxLon)
	case q.Scope.Around != nil:
		a := q.Scope.Around
		scope = fmt.Sprintf("(around:%g,%g,%g)", a.Radius, a.Lat, a.Lon)
	case q.Scope.AreaID != 0:
		pre = fmt.Sprintf("area(id:%d)->.a;\n", q.Scope.AreaID)
		scope = "(area.a)"
	default:
		return "", fmt.Errorf("a search area is required (bbox, around a point, or a named area)")
	}

	types := q.Types
	if len(types) == 0 {
		types = []osm.Type{"nwr"}
	}
	var filters strings.Builder
	for _, f := range q.Filters {
		filters.WriteString(f.QL())
	}

	timeout := q.Timeout
	if timeout <= 0 {
		timeout = 60
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[out:json][timeout:%d];\n%s(\n", timeout, pre)
	for _, t := range types {
		fmt.Fprintf(&b, "  %s%s%s;\n", t, filters.String(), scope)
	}
	b.WriteString(");\n")
	limit := ""
	if q.Limit > 0 {
		limit = fmt.Sprintf(" %d", q.Limit)
	}
	fmt.Fprintf(&b, "out body%s;\n>;\nout skel qt;", limit)
	return b.String(), nil
}
