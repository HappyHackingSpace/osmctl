package cli

import (
	"fmt"
	"strconv"

	"github.com/cli/browser"
	"github.com/paulmach/osm"
	"github.com/spf13/cobra"

	"github.com/happyhackingspace/osmctl/internal/api"
	"github.com/happyhackingspace/osmctl/internal/geo"
	"github.com/happyhackingspace/osmctl/internal/osmref"
	"github.com/happyhackingspace/osmctl/internal/output"
)

func newGetCmd(a *app) *cobra.Command {
	var (
		full    bool
		version int
	)
	cmd := &cobra.Command{
		Use:   "get <element>...",
		Short: "Show nodes, ways and relations",
		Long: `Show the current state of one or more elements.

Elements can be written as node/123, n123, w42, r7, or pasted as an
openstreetmap.org URL.`,
		Example: `  osmctl get node/2187164523
  osmctl get w42 --full -o geojson > way.geojson
  osmctl get https://www.openstreetmap.org/relation/7 -o json
  osmctl get n123 --version 2`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := a.client()
			all := &osm.OSM{}
			for _, arg := range args {
				id, err := osmref.Parse(arg)
				if err != nil {
					return err
				}
				var o *osm.OSM
				if version > 0 {
					o, err = c.ElementVersion(cmd.Context(), id, version)
				} else {
					// GeoJSON of a way or relation is meaningless without its nodes.
					o, err = c.Element(cmd.Context(), id, full || a.format == output.GeoJSON)
				}
				if err != nil {
					return notFoundHint(err, id.String())
				}
				merge(all, o)
			}
			return output.Data(a.out, all, a.format)
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "include every node and member needed to draw ways and relations")
	cmd.Flags().IntVar(&version, "version", 0, "show a specific historical version")
	return cmd
}

func newHistoryCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "history <element>",
		Short:   "Show every version of an element",
		Example: "  osmctl history node/2187164523\n  osmctl history w42 -o json | jq '.elements[].user'",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := osmref.Parse(args[0])
			if err != nil {
				return err
			}
			o, err := a.client().History(cmd.Context(), id)
			if err != nil {
				return notFoundHint(err, id.String())
			}
			return output.Data(a.out, o, a.format)
		},
	}
}

func newMapCmd(a *app) *cobra.Command {
	var bbox string
	cmd := &cobra.Command{
		Use:   "map --bbox <minlon,minlat,maxlon,maxlat>",
		Short: "Download all map data in a small area",
		Long: `Download every node, way and relation in a bounding box from the OSM API.

The API caps this at 50,000 nodes and 0.25 square degrees. For larger areas
use ` + "`osmctl query`" + ` (Overpass) or a regional extract with ` + "`osmctl data`" + `.`,
		Example: "  osmctl map --bbox 40.2290,37.9100,40.2330,37.9130 -o xml > area.osm",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			b, err := geo.ParseBBox(bbox)
			if err != nil {
				return err
			}
			o, err := a.client().Map(cmd.Context(), b)
			if err != nil {
				return err
			}
			return output.Data(a.out, o, a.format)
		},
	}
	cmd.Flags().StringVar(&bbox, "bbox", "", "bounding box as minlon,minlat,maxlon,maxlat")
	_ = cmd.MarkFlagRequired("bbox")
	return cmd
}

func newUserCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "user <id>",
		Short: "Show a user's public profile",
		Long: `Show a user's public profile by numeric id.

The OSM API looks users up by id only. ` + "`osmctl auth status`" + ` shows your own.`,
		Example: "  osmctl user 1",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("user id must be numeric: %q", args[0])
			}
			u, err := a.client().User(cmd.Context(), osm.UserID(id))
			if err != nil {
				return notFoundHint(err, fmt.Sprintf("user/%d", id))
			}
			return output.User(a.out, u, a.format)
		},
	}
}

func newBrowseCmd(a *app) *cobra.Command {
	var printOnly bool
	cmd := &cobra.Command{
		Use:     "browse <element|changeset/ID|note/ID>",
		Short:   "Open an element, changeset or note on the website",
		Example: "  osmctl browse w42\n  osmctl browse changeset/151234567 --print",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			path, err := webPath(args[0])
			if err != nil {
				return err
			}
			u := a.srv.Web + "/" + path
			if printOnly {
				_, err = fmt.Fprintln(a.out, u)
				return err
			}
			return browser.OpenURL(u)
		},
	}
	cmd.Flags().BoolVarP(&printOnly, "print", "p", false, "print the URL instead of opening it")
	return cmd
}

func webPath(ref string) (string, error) {
	var kind string
	var n int64
	if _, err := fmt.Sscanf(ref, "changeset/%d", &n); err == nil {
		kind = "changeset"
	} else if _, err := fmt.Sscanf(ref, "note/%d", &n); err == nil {
		kind = "note"
	}
	if kind != "" {
		return fmt.Sprintf("%s/%d", kind, n), nil
	}
	id, err := osmref.Parse(ref)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func merge(dst, src *osm.OSM) {
	dst.Nodes = append(dst.Nodes, src.Nodes...)
	dst.Ways = append(dst.Ways, src.Ways...)
	dst.Relations = append(dst.Relations, src.Relations...)
}

func notFoundHint(err error, what string) error {
	if api.IsNotFound(err) {
		return fmt.Errorf("%s not found", what)
	}
	if api.IsGone(err) {
		return fmt.Errorf("%s has been deleted (see `osmctl history %s`)", what, what)
	}
	return err
}
