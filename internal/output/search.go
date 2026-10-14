package output

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

// Found renders search results. In text form it lists only the tagged
// elements, since the untagged nodes are just geometry for the others.
func Found(w io.Writer, d *osm.Data, f Format) error {
	if f != Text {
		return Data(w, d, f)
	}
	coords := make(map[int64][2]float64, len(d.Nodes))
	for _, n := range d.Nodes {
		coords[n.ID] = [2]float64{n.Lat, n.Lon}
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	rows := 0
	for _, el := range d.Elements() {
		ts := osm.TagsOf(el)
		if len(ts) == 0 {
			continue
		}
		if rows == 0 {
			fmt.Fprintln(tw, "ELEMENT\tNAME\tLOCATION\tTAGS")
		}
		rows++
		loc := "-"
		switch v := el.(type) {
		case *osm.Node:
			loc = fmt.Sprintf("%.6f,%.6f", v.Lat, v.Lon)
		case *osm.Way:
			loc = centroid(v.Nodes, coords)
		}
		name := ts.Find("name")
		if name == "" {
			name = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", el.ElementID(), name, loc, summary(ts))
	}
	if rows == 0 {
		fmt.Fprintln(tw, "no matches")
	} else {
		fmt.Fprintf(tw, "\n%d results\n", rows)
	}
	return tw.Flush()
}

// centroid is the mean of the way's node coordinates: good enough to point at
// a place, not a true polygon centroid.
func centroid(nodes []int64, coords map[int64][2]float64) string {
	var lat, lon float64
	var n int
	for _, id := range nodes {
		if c, ok := coords[id]; ok {
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

func summary(ts osm.Tags) string {
	var parts []string
	for _, t := range ts {
		if t.Key == "name" || strings.HasPrefix(t.Key, "name:") {
			continue
		}
		parts = append(parts, t.Key+"="+t.Value)
	}
	sort.Strings(parts)
	return oneLine(strings.Join(parts, " "), 70)
}
