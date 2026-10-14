// Package output renders OSM data for people (text) and for tools (json, xml, geojson).
package output

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/happyhackingspace/osmctl/internal/osm"
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
func Data(w io.Writer, d *osm.Data, f Format) error {
	switch f {
	case JSON:
		return WriteJSON(w, d)
	case XML:
		return osm.WriteXML(w, d)
	case GeoJSON:
		return WriteJSON(w, osm.GeoJSON(d))
	default:
		return dataText(w, d)
	}
}

func dataText(w io.Writer, d *osm.Data) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	els := d.Elements()
	for i, el := range els {
		if i > 0 {
			fmt.Fprintln(tw)
		}
		header(tw, el.ElementID(), osm.MetaOf(el))
		switch v := el.(type) {
		case *osm.Node:
			fmt.Fprintf(tw, "  location\t%.7f, %.7f\n", v.Lat, v.Lon)
		case *osm.Way:
			fmt.Fprintf(tw, "  nodes\t%d %s\n", len(v.Nodes), refs(v.Nodes))
		case *osm.Relation:
			fmt.Fprintf(tw, "  members\t%d\n", len(v.Members))
			for _, m := range v.Members {
				role := m.Role
				if role == "" {
					role = "-"
				}
				fmt.Fprintf(tw, "    %s\t%s\n", m.ElementID(), role)
			}
		}
		tags(tw, osm.TagsOf(el))
	}
	if len(els) == 0 {
		fmt.Fprintln(tw, "no elements")
	}
	return tw.Flush()
}

func header(w io.Writer, id osm.ID, m *osm.Meta) {
	line := id.String()
	if m.Version > 0 {
		line += fmt.Sprintf("  v%d", m.Version)
	}
	if !m.Timestamp.IsZero() {
		line += "  " + m.Timestamp.UTC().Format("2006-01-02 15:04")
	}
	if m.User != "" {
		line += "  by " + m.User
	}
	if m.Changeset > 0 {
		line += fmt.Sprintf("  (changeset %d)", m.Changeset)
	}
	if !m.Visible {
		line += "  [deleted]"
	}
	fmt.Fprintln(w, line)
}

func tags(w io.Writer, ts osm.Tags) {
	for _, t := range ts.Sorted() {
		fmt.Fprintf(w, "  %s\t%s\n", t.Key, t.Value)
	}
}

func refs(ids []int64) string {
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

func date(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format("2006-01-02")
}

func oneLine(s string, limit int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > limit {
		return string(r[:limit-1]) + "…"
	}
	return s
}
