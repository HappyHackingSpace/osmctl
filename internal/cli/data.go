package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/happyhackingspace/osmctl/internal/geo"
	"github.com/happyhackingspace/osmctl/internal/geofabrik"
	"github.com/happyhackingspace/osmctl/internal/osm"
	"github.com/happyhackingspace/osmctl/internal/osmfile"
	"github.com/happyhackingspace/osmctl/internal/output"
	"github.com/happyhackingspace/osmctl/internal/tagfilter"
)

func newDataCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "data",
		Short: "Work with local .osm and .osm.pbf files",
		Long: `Work offline with OSM files: download regional extracts, inspect them,
and cut out what you need by tag or area.

Supported inputs: .osm.pbf, .osm, .osm.gz and .osm.bz2.`,
	}
	cmd.AddCommand(newDataRegionsCmd(a), newDataDownloadCmd(a), newDataInfoCmd(a), newDataFilterCmd(a))
	return cmd
}

func (a *app) geofabrik() *geofabrik.Client {
	return &geofabrik.Client{HTTP: a.http(), Index: geofabrik.IndexURL}
}

func newDataRegionsCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "regions [search]",
		Short:   "List regional extracts available from Geofabrik",
		Example: "  osmctl data regions\n  osmctl data regions turkey",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			regions, err := a.geofabrik().Regions(cmd.Context())
			if err != nil {
				return err
			}
			if len(args) == 1 {
				regions = geofabrik.Search(regions, args[0])
			}
			if a.format != output.Text {
				return output.WriteJSON(a.out, regions)
			}
			tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tPARENT")
			for _, r := range regions {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", r.ID, r.Name, r.Parent)
			}
			return tw.Flush()
		},
	}
}

func newDataDownloadCmd(a *app) *cobra.Command {
	var dest string
	cmd := &cobra.Command{
		Use:   "download <region>",
		Short: "Download a regional extract as .osm.pbf",
		Long: `Download the latest extract of a region from Geofabrik, updated daily.
Find region ids with ` + "`osmctl data regions`" + `.`,
		Example: "  osmctl data download turkey\n  osmctl data download iraq -O data/iraq.osm.pbf",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := a.geofabrik()
			regions, err := c.Regions(cmd.Context())
			if err != nil {
				return err
			}
			r, err := geofabrik.Find(regions, args[0])
			if err != nil {
				return err
			}
			if dest == "" {
				dest = r.ID + "-latest.osm.pbf"
			}
			// Write next to the destination and rename at the end, so an
			// interrupted download never leaves a truncated file behind.
			tmp, err := os.CreateTemp(filepath.Dir(dest), ".osmctl-download-*")
			if err != nil {
				return err
			}
			defer func() { _ = os.Remove(tmp.Name()) }()

			fmt.Fprintf(a.errOut, "Downloading %s from %s\n", r.Name, r.PBF)
			start := time.Now()
			var lastReport time.Time
			n, err := c.Download(cmd.Context(), r, tmp, func(done, total int64) {
				if time.Since(lastReport) < time.Second {
					return
				}
				lastReport = time.Now()
				msg := size(done)
				if total > 0 {
					msg = fmt.Sprintf("%s of %s (%d%%)", size(done), size(total), done*100/total)
				}
				fmt.Fprintf(a.errOut, "\r  %-36s", msg)
			})
			if cerr := tmp.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
			if err := os.Rename(tmp.Name(), dest); err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "\r  %-36s\n", fmt.Sprintf("%s in %s", size(n), time.Since(start).Round(time.Second)))
			_, err = fmt.Fprintln(a.out, dest)
			return err
		},
	}
	cmd.Flags().StringVarP(&dest, "output-file", "O", "", "where to save (default <region>-latest.osm.pbf)")
	return cmd
}

func size(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func newDataInfoCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "info <file>",
		Short:   "Count elements and show the extent and age of a file",
		Example: "  osmctl data info turkey-latest.osm.pbf",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := osmfile.Info(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if a.format != output.Text {
				return output.WriteJSON(a.out, st)
			}
			tw := tabwriter.NewWriter(a.out, 0, 4, 2, ' ', 0)
			fmt.Fprintf(tw, "nodes\t%d\nways\t%d\nrelations\t%d\n", st.Nodes, st.Ways, st.Relations)
			if b := st.Bounds; b != nil {
				fmt.Fprintf(tw, "bbox\t%g,%g,%g,%g\n", b.MinLon, b.MinLat, b.MaxLon, b.MaxLat)
			}
			if !st.FirstEdit.IsZero() {
				fmt.Fprintf(tw, "edits\t%s to %s\n", st.FirstEdit.UTC().Format(time.DateOnly), st.LastEdit.UTC().Format(time.DateOnly))
			}
			if !st.ReplicationTimestamp.IsZero() {
				fmt.Fprintf(tw, "data as of\t%s\n", st.ReplicationTimestamp.UTC().Format(time.RFC3339))
			}
			if st.WritingProgram != "" {
				fmt.Fprintf(tw, "written by\t%s\n", st.WritingProgram)
			}
			return tw.Flush()
		},
	}
}

func newDataFilterCmd(a *app) *cobra.Command {
	var (
		bbox  string
		types []string
	)
	cmd := &cobra.Command{
		Use:     "filter <file> [filter]... [--bbox ...]",
		Aliases: []string{"extract"},
		Short:   "Extract features by tag and/or area from a file",
		Long: `Extract features from a local file by tag (same filters as ` + "`osmctl find`" + `),
by bounding box, or both. Ways come with every node they need, so the
result converts cleanly:

  -o xml      an .osm file for JOSM or other tools
  -o geojson  for QGIS, uMap, Leaflet and friends
  -o json     OSM JSON
  -o text     a table of what matched (default)

With only --bbox, everything touching the box is extracted.`,
		Example: `  osmctl data filter turkey-latest.osm.pbf amenity=drinking_water --bbox 40.1,37.8,40.3,38.0
  osmctl data filter region.osm.pbf highway=bus_stop -o geojson > stops.geojson
  osmctl data extract region.osm.pbf --bbox 40.20,37.90,40.25,37.93 -o xml > old-city.osm`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := osmfile.Query{}
			var err error
			if q.Filters, err = tagfilter.ParseAll(args[1:]); err != nil {
				return err
			}
			if bbox != "" {
				if q.Bounds, err = geo.ParseBBox(bbox); err != nil {
					return err
				}
			}
			if len(q.Filters) == 0 && q.Bounds == nil {
				return errors.New("give at least one tag filter or --bbox (to convert a whole file, use --bbox with its extent from `osmctl data info`)")
			}
			for _, t := range types {
				switch osm.Type(t) {
				case osm.TypeNode, osm.TypeWay, osm.TypeRelation:
					q.Types = append(q.Types, osm.Type(t))
				default:
					return fmt.Errorf("unknown type %q (use node, way, relation)", t)
				}
			}
			o, err := osmfile.Extract(cmd.Context(), args[0], q)
			if err != nil {
				return err
			}
			return output.Found(a.out, o, a.format)
		},
	}
	cmd.Flags().StringVar(&bbox, "bbox", "", "only features touching minlon,minlat,maxlon,maxlat")
	cmd.Flags().StringSliceVarP(&types, "type", "t", nil, "limit to node, way and/or relation")
	return cmd
}
