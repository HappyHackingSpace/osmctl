package output

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/paulmach/osm"
)

// structured renders anything that is not map data. GeoJSON only makes sense
// for things with a location, so callers that have none fall back to JSON.
func structured(w io.Writer, f Format, v any, text func(*tabwriter.Writer)) error {
	switch f {
	case JSON, GeoJSON:
		return WriteJSON(w, v)
	case XML:
		return WriteXML(w, v)
	default:
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		text(tw)
		return tw.Flush()
	}
}

func Changesets(w io.Writer, cs osm.Changesets, f Format) error {
	return structured(w, f, &osm.OSM{Changesets: cs}, func(tw *tabwriter.Writer) {
		if len(cs) == 0 {
			fmt.Fprintln(tw, "no changesets")
			return
		}
		fmt.Fprintln(tw, "ID\tCREATED\tUSER\tCHANGES\tCOMMENT")
		for _, c := range cs {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%s\n", c.ID, date(c.CreatedAt), c.User, c.ChangesCount, oneLine(c.Tags.Find("comment"), 60))
		}
	})
}

func Changeset(w io.Writer, c *osm.Changeset, f Format) error {
	return structured(w, f, &osm.OSM{Changesets: osm.Changesets{c}}, func(tw *tabwriter.Writer) {
		state := "closed " + date(c.ClosedAt)
		if c.Open {
			state = "open"
		}
		fmt.Fprintf(tw, "changeset/%d  by %s  %s  (%s)\n", c.ID, c.User, date(c.CreatedAt), state)
		fmt.Fprintf(tw, "  changes\t%d\n", c.ChangesCount)
		if c.MinLat != 0 || c.MaxLat != 0 {
			fmt.Fprintf(tw, "  bbox\t%g,%g,%g,%g\n", c.MinLon, c.MinLat, c.MaxLon, c.MaxLat)
		}
		tags(tw, c.Tags)
		if c.Discussion != nil {
			for _, cm := range c.Discussion.Comments {
				fmt.Fprintf(tw, "\n  %s  %s:\n    %s\n", date(cm.Timestamp), cm.User, strings.ReplaceAll(cm.Text, "\n", "\n    "))
			}
		}
	})
}

func Notes(w io.Writer, ns osm.Notes, f Format) error {
	return structured(w, f, &osm.OSM{Notes: ns}, func(tw *tabwriter.Writer) {
		if len(ns) == 0 {
			fmt.Fprintln(tw, "no notes")
			return
		}
		fmt.Fprintln(tw, "ID\tSTATUS\tCREATED\tLOCATION\tTEXT")
		for _, n := range ns {
			fmt.Fprintf(tw, "%d\t%s\t%s\t%.5f,%.5f\t%s\n", n.ID, n.Status, date(n.DateCreated.Time), n.Lat, n.Lon, oneLine(firstComment(n), 60))
		}
	})
}

func Note(w io.Writer, n *osm.Note, f Format) error {
	return structured(w, f, &osm.OSM{Notes: osm.Notes{n}}, func(tw *tabwriter.Writer) {
		fmt.Fprintf(tw, "note/%d  %s  at %.6f, %.6f\n", n.ID, n.Status, n.Lat, n.Lon)
		for _, c := range n.Comments {
			who := c.User
			if who == "" {
				who = "anonymous"
			}
			fmt.Fprintf(tw, "\n  %s  %s %s:\n    %s\n", date(c.Date.Time), who, c.Action, strings.ReplaceAll(c.Text, "\n", "\n    "))
		}
	})
}

func User(w io.Writer, u *osm.User, f Format) error {
	return structured(w, f, &osm.OSM{Users: osm.Users{u}}, func(tw *tabwriter.Writer) {
		fmt.Fprintf(tw, "user/%d  %s\n", u.ID, u.Name)
		fmt.Fprintf(tw, "  joined\t%s\n", date(u.CreatedAt))
		fmt.Fprintf(tw, "  changesets\t%d\n", u.Changesets.Count)
		fmt.Fprintf(tw, "  traces\t%d\n", u.Traces.Count)
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
