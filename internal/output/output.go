// Package output renders OSM data for people (text) and for tools (json, xml, geojson).
package output

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmgeojson"
)

type Format string

const (
	Text    Format = "text"
	JSON    Format = "json"
	XML     Format = "xml"
	GeoJSON Format = "geojson"
)

var Formats = []Format{Text, JSON, XML, GeoJSON}

func ParseFormat(s string) (Format, error) {
	for _, f := range Formats {
		if string(f) == s {
			return f, nil
		}
	}
	return "", fmt.Errorf("unknown format %q (use text, json, xml or geojson)", s)
}

func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func WriteXML(w io.Writer, v any) error {
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// Data renders map data: elements, histories, query and map results.
func Data(w io.Writer, o *osm.OSM, f Format) error {
	switch f {
	case JSON:
		return WriteJSON(w, o)
	case XML:
		// JOSM and other editors reject an <osm> root without a version.
		cp := *o
		if cp.Version == "" {
			cp.Version = "0.6"
		}
		if cp.Generator == "" {
			cp.Generator = "osmctl"
		}
		return WriteXML(w, &cp)
	case GeoJSON:
		fc, err := osmgeojson.Convert(o, osmgeojson.NoMeta(true))
		if err != nil {
			return err
		}
		return WriteJSON(w, fc)
	default:
		return dataText(w, o)
	}
}

func dataText(w io.Writer, o *osm.OSM) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	first := true
	block := func() {
		if !first {
			fmt.Fprintln(tw)
		}
		first = false
	}
	for _, n := range o.Nodes {
		block()
		header(tw, n.FeatureID(), n.Version, n.Timestamp, n.User, n.ChangesetID, n.Visible)
		fmt.Fprintf(tw, "  location\t%.7f, %.7f\n", n.Lat, n.Lon)
		tags(tw, n.Tags)
	}
	for _, way := range o.Ways {
		block()
		header(tw, way.FeatureID(), way.Version, way.Timestamp, way.User, way.ChangesetID, way.Visible)
		fmt.Fprintf(tw, "  nodes\t%d %s\n", len(way.Nodes), refs(way.Nodes.NodeIDs()))
		tags(tw, way.Tags)
	}
	for _, r := range o.Relations {
		block()
		header(tw, r.FeatureID(), r.Version, r.Timestamp, r.User, r.ChangesetID, r.Visible)
		fmt.Fprintf(tw, "  members\t%d\n", len(r.Members))
		for _, m := range r.Members {
			role := m.Role
			if role == "" {
				role = "-"
			}
			fmt.Fprintf(tw, "    %s/%d\t%s\n", m.Type, m.Ref, role)
		}
		tags(tw, r.Tags)
	}
	if first {
		fmt.Fprintln(tw, "no elements")
	}
	return tw.Flush()
}

func header(w io.Writer, id osm.FeatureID, version int, ts time.Time, user string, cs osm.ChangesetID, visible bool) {
	line := id.String()
	if version > 0 {
		line += fmt.Sprintf("  v%d", version)
	}
	if !ts.IsZero() {
		line += "  " + ts.UTC().Format("2006-01-02 15:04")
	}
	if user != "" {
		line += "  by " + user
	}
	if cs > 0 {
		line += fmt.Sprintf("  (changeset %d)", cs)
	}
	if version > 0 && !visible {
		line += "  [deleted]"
	}
	fmt.Fprintln(w, line)
}

func tags(w io.Writer, ts osm.Tags) {
	sorted := append(osm.Tags(nil), ts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })
	for _, t := range sorted {
		fmt.Fprintf(w, "  %s\t%s\n", t.Key, t.Value)
	}
}

func refs(ids []osm.NodeID) string {
	const show = 6
	parts := make([]string, 0, show+1)
	for i, id := range ids {
		if i == show {
			parts = append(parts, fmt.Sprintf("… +%d", len(ids)-show))
			break
		}
		parts = append(parts, fmt.Sprintf("n%d", id))
	}
	return "(" + strings.Join(parts, " ") + ")"
}
