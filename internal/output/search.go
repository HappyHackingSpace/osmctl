package output

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/paulmach/osm"
)

// Found renders search results. In text form it lists only the tagged
// elements, since the untagged nodes are just geometry for the others.
func Found(w io.Writer, o *osm.OSM, f Format) error {
	if f != Text {
		return Data(w, o, f)
	}
	coords := make(map[osm.NodeID][2]float64, len(o.Nodes))
	for _, n := range o.Nodes {
		coords[n.ID] = [2]float64{n.Lat, n.Lon}
	}

	type row struct{ id, name, loc, tags string }
	var rows []row
	add := func(id osm.FeatureID, tags osm.Tags, loc string) {
		if len(tags) == 0 {
			return
		}
		rows = append(rows, row{id.String(), tags.Find("name"), loc, summary(tags)})
	}
	for _, n := range o.Nodes {
		add(n.FeatureID(), n.Tags, fmt.Sprintf("%.6f,%.6f", n.Lat, n.Lon))
	}
	for _, way := range o.Ways {
		add(way.FeatureID(), way.Tags, centroid(way.Nodes, coords))
	}
	for _, r := range o.Relations {
		add(r.FeatureID(), r.Tags, "-")
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if len(rows) == 0 {
		fmt.Fprintln(tw, "no matches")
		return tw.Flush()
	}
	fmt.Fprintln(tw, "ELEMENT\tNAME\tLOCATION\tTAGS")
	for _, r := range rows {
		name := r.name
		if name == "" {
			name = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.id, name, r.loc, r.tags)
	}
	fmt.Fprintf(tw, "\n%d results\n", len(rows))
	return tw.Flush()
}

// centroid is the mean of the way's node coordinates: good enough to point at
// a place, not a true polygon centroid.
func centroid(nodes osm.WayNodes, coords map[osm.NodeID][2]float64) string {
	var lat, lon float64
	var n int
	for _, wn := range nodes {
		if c, ok := coords[wn.ID]; ok {
			lat += c[0]
			lon += c[1]
			n++
		}
	}
	if n == 0 {
		return "-"
	}
	return fmt.Sprintf("%.6f,%.6f", lat/float64(n), lon/float64(n))
}

func summary(tags osm.Tags) string {
	var parts []string
	for _, t := range tags {
		if t.Key == "name" || strings.HasPrefix(t.Key, "name:") {
			continue
		}
		parts = append(parts, t.Key+"="+t.Value)
	}
	sort.Strings(parts)
	return oneLine(strings.Join(parts, " "), 70)
}
