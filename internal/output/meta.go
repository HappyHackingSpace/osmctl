package output

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/happyhackingspace/osmctl/internal/osm"
)

// structured renders anything that is not map data. GeoJSON only makes sense
// for things with a location, so callers that have none fall back to JSON.
func structured(w io.Writer, f Format, jsonValue any, xmlValue *osm.Data, text func(*tabwriter.Writer)) error {
	switch f {
	case JSON, GeoJSON:
		return WriteJSON(w, jsonValue)
	case XML:
		return osm.WriteXML(w, xmlValue)
	default:
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		text(tw)
		return tw.Flush()
	}
}

func Changesets(w io.Writer, cs []*osm.Changeset, f Format) error {
	if cs == nil {
		cs = []*osm.Changeset{}
	}
	return structured(w, f, cs, &osm.Data{Changesets: cs}, func(tw *tabwriter.Writer) {
		if len(cs) == 0 {
			fmt.Fprintln(tw, "no changesets")
			return
		}
		fmt.Fprintln(tw, "ID\tCREATED\tUSER\tCHANGES\tCOMMENT")
		for _, c := range cs {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%s\n", c.ID, date(c.CreatedAt), c.User, c.Changes, oneLine(c.Tags.Find("comment"), 60))
		}
	})
}

func Changeset(w io.Writer, c *osm.Changeset, f Format) error {
	return structured(w, f, c, &osm.Data{Changesets: []*osm.Changeset{c}}, func(tw *tabwriter.Writer) {
		state := "closed " + date(c.ClosedAt)
		if c.Open {
			state = "open"
		}
		fmt.Fprintf(tw, "changeset/%d  by %s  %s  (%s)\n", c.ID, c.User, date(c.CreatedAt), state)
		fmt.Fprintf(tw, "  changes\t%d\n", c.Changes)
		if b := c.Bounds; b != nil {
			fmt.Fprintf(tw, "  bbox\t%g,%g,%g,%g\n", b.MinLon, b.MinLat, b.MaxLon, b.MaxLat)
		}
		tags(tw, c.Tags)
		for _, cm := range c.Discussion {
			fmt.Fprintf(tw, "\n  %s  %s:\n    %s\n", date(cm.Date), cm.User, strings.ReplaceAll(cm.Text, "\n", "\n    "))
		}
	})
}

func Notes(w io.Writer, ns []*osm.Note, f Format) error {
	if ns == nil {
		ns = []*osm.Note{}
	}
	return structured(w, f, ns, &osm.Data{Notes: ns}, func(tw *tabwriter.Writer) {
		if len(ns) == 0 {
			fmt.Fprintln(tw, "no notes")
			return
		}
		fmt.Fprintln(tw, "ID\tSTATUS\tCREATED\tLOCATION\tTEXT")
		for _, n := range ns {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%.5f,%.5f\t%s\n", n.ID, n.Status, date(n.Created), n.Lat, n.Lon, oneLine(firstComment(n), 60))
		}
	})
}

func Note(w io.Writer, n *osm.Note, f Format) error {
	return structured(w, f, n, &osm.Data{Notes: []*osm.Note{n}}, func(tw *tabwriter.Writer) {
		fmt.Fprintf(tw, "note/%d  %s  at %.6f, %.6f\n", n.ID, n.Status, n.Lat, n.Lon)
		for _, c := range n.Comments {
			who := c.User
			if who == "" {
				who = "anonymous"
			}
			fmt.Fprintf(tw, "\n  %s  %s %s:\n    %s\n", date(c.Date), who, c.Action, strings.ReplaceAll(c.Text, "\n", "\n    "))
		}
	})
}

func User(w io.Writer, u *osm.User, f Format) error {
	return structured(w, f, u, &osm.Data{Users: []*osm.User{u}}, func(tw *tabwriter.Writer) {
		fmt.Fprintf(tw, "user/%d  %s\n", u.ID, u.Name)
		fmt.Fprintf(tw, "  joined\t%s\n", date(u.Created))
		fmt.Fprintf(tw, "  changesets\t%d\n", u.Changesets)
		fmt.Fprintf(tw, "  traces\t%d\n", u.Traces)
		if d := strings.TrimSpace(u.Description); d != "" {
			fmt.Fprintf(tw, "  about\t%s\n", oneLine(d, 100))
		}
	})
}

func firstComment(n *osm.Note) string {
	if len(n.Comments) == 0 {
		return ""
	}
	return n.Comments[0].Text
}

// Change renders an osmChange. XML is a .osc file; JSON keeps the
// create/modify/delete split.
func Change(w io.Writer, c *osm.Change, f Format) error {
	switch f {
	case XML:
		return osm.WriteChangeXML(w, c)
	case JSON, GeoJSON:
		return WriteJSON(w, c)
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	empty := true
	for _, sec := range []struct {
		name string
		d    *osm.Data
	}{{"create", c.Create}, {"modify", c.Modify}, {"delete", c.Delete}} {
		for _, el := range sec.d.Elements() {
			empty = false
			fmt.Fprintf(tw, "%s\t%s\n", sec.name, el.ElementID())
		}
	}
	if empty {
		fmt.Fprintln(tw, "no changes")
	}
	return tw.Flush()
}
