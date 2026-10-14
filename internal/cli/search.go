package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/paulmach/osm"
	"github.com/spf13/cobra"

	"github.com/happyhackingspace/osmctl/internal/geo"
	"github.com/happyhackingspace/osmctl/internal/nominatim"
	"github.com/happyhackingspace/osmctl/internal/output"
	"github.com/happyhackingspace/osmctl/internal/overpass"
	"github.com/happyhackingspace/osmctl/internal/tagfilter"
)

func (a *app) overpass() *overpass.Client {
	return &overpass.Client{URL: a.srv.Overpass, HTTP: a.http()}
}

func (a *app) nominatim() *nominatim.Client {
	return &nominatim.Client{URL: a.srv.Nominatim, HTTP: a.http()}
}

func newQueryCmd(a *app) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "query [overpass-ql]",
		Short: "Run an Overpass QL query",
		Long: `Run an Overpass QL query, given as an argument, with --file, or on stdin.

JSON and XML responses are parsed, so -o text, json, xml and geojson all
work. For GeoJSON, ways and relations need their nodes in the result: end
the query with "out body; >; out skel qt;". Other outputs, such as
[out:csv], are printed as Overpass returns them.

For simple tag searches, ` + "`osmctl find`" + ` writes the query for you.`,
		Example: `  osmctl query '[out:json];node["amenity"="cafe"](around:500,37.91,40.23);out;'
  osmctl query -f stations.overpassql -o geojson > stations.geojson
  echo 'node(1);out;' | osmctl query`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ql, err := readQuery(args, file, a.in)
			if err != nil {
				return err
			}
			res, err := a.overpass().Query(cmd.Context(), ql)
			if err != nil {
				return err
			}
			if res.Raw != nil {
				_, err = a.out.Write(res.Raw)
				return err
			}
			return output.Data(a.out, res.Data, a.format)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "read the query from a file")
	return cmd
}

func readQuery(args []string, file string, stdin io.Reader) (string, error) {
	var b []byte
	var err error
	switch {
	case len(args) == 1 && file != "":
		return "", errors.New("give the query as an argument or with --file, not both")
	case len(args) == 1:
		return args[0], nil
	case file != "":
		b, err = os.ReadFile(file)
	default:
		b, err = io.ReadAll(stdin)
	}
	if err != nil {
		return "", err
	}
	q := strings.TrimSpace(string(b))
	if q == "" {
		return "", errors.New("empty query")
	}
	return q, nil
}

func newFindCmd(a *app) *cobra.Command {
	var (
		near, bbox, in string
		radius         float64
		types          []string
		limit, timeout int
		show           bool
	)
	cmd := &cobra.Command{
		Use:   "find <filter>... (--near lat,lon | --bbox ... | --in place)",
		Short: "Find map features by tag, near a point, in a box or in a named place",
		Long: `Find map features by tag without writing Overpass QL.

Every filter must match. Quote filters containing !, which shells
otherwise treat as history expansion:
  key            the tag is present         amenity
  !key           the tag is absent          '!name'
  key=value      exact match                amenity=cafe
  key!=value     anything but               'cuisine!=pizza'
  key~regex      regular expression         name~^Kafe
  key!~regex     does not match             'name!~bar'

--in looks the place up with Nominatim and searches inside its boundary.`,
		Example: `  osmctl find amenity=cafe --near 37.9144,40.2306 --radius 500
  osmctl find amenity=drinking_water --in "Diyarbakır"
  osmctl find highway=bus_stop '!name' --bbox 40.20,37.90,40.25,37.93
  osmctl find tourism=museum --in "Berlin" -o geojson > museums.geojson
  osmctl find shop=bakery --in "Amed" --show-query`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			q := overpass.FindQuery{Limit: limit, Timeout: timeout}
			filters, err := tagfilter.ParseAll(args)
			if err != nil {
				return err
			}
			q.Filters = filters
			for _, t := range types {
				switch osm.Type(t) {
				case osm.TypeNode, osm.TypeWay, osm.TypeRelation:
					q.Types = append(q.Types, osm.Type(t))
				default:
					return fmt.Errorf("unknown type %q (use node, way, relation)", t)
				}
			}
			scope, err := a.findScope(cmd.Context(), near, radius, bbox, in)
			if err != nil {
				return err
			}
			q.Scope = scope

			ql, err := q.Build()
			if err != nil {
				return err
			}
			if show {
				_, err = fmt.Fprintln(a.out, ql)
				return err
			}
			res, err := a.overpass().Query(cmd.Context(), ql)
			if err != nil {
				return err
			}
			return output.Found(a.out, res.Data, a.format)
		},
	}
	f := cmd.Flags()
	f.StringVar(&near, "near", "", "search around lat,lon")
	f.Float64VarP(&radius, "radius", "r", 1000, "radius in meters for --near")
	f.StringVar(&bbox, "bbox", "", "search in minlon,minlat,maxlon,maxlat")
	f.StringVar(&in, "in", "", "search inside a named place (city, district, country)")
	f.StringSliceVarP(&types, "type", "t", nil, "limit to node, way and/or relation (default all)")
	f.IntVarP(&limit, "limit", "n", 0, "maximum number of results (default no limit)")
	f.IntVar(&timeout, "timeout", 60, "Overpass timeout in seconds")
	f.BoolVar(&show, "show-query", false, "print the generated Overpass QL instead of running it")
	cmd.MarkFlagsMutuallyExclusive("near", "bbox", "in")
	cmd.MarkFlagsOneRequired("near", "bbox", "in")
	return cmd
}

func (a *app) findScope(ctx context.Context, near string, radius float64, bbox, in string) (overpass.Scope, error) {
	switch {
	case near != "":
		p, err := geo.ParsePoint(near)
		if err != nil {
			return overpass.Scope{}, err
		}
		if radius <= 0 {
			return overpass.Scope{}, errors.New("--radius must be positive")
		}
		return overpass.Scope{Around: &overpass.Around{Lat: p.Lat, Lon: p.Lon, Radius: radius}}, nil
	case bbox != "":
		b, err := geo.ParseBBox(bbox)
		return overpass.Scope{Bounds: b}, err
	default:
		places, err := a.nominatim().Search(ctx, in, nominatim.SearchOptions{Limit: 5})
		if err != nil {
			return overpass.Scope{}, err
		}
		for _, p := range places {
			id, ok := p.FeatureID()
			if !ok {
				continue
			}
			if areaID, err := overpass.AreaIDFor(id); err == nil {
				fmt.Fprintf(a.errOut, "Searching in %s (%s)\n", p.DisplayName, id)
				return overpass.Scope{AreaID: areaID}, nil
			}
		}
		return overpass.Scope{}, fmt.Errorf("no area named %q found; try --bbox or --near", in)
	}
}

func newGeocodeCmd(a *app) *cobra.Command {
	var opts nominatim.SearchOptions
	cmd := &cobra.Command{
		Use:   "geocode <place or address>",
		Short: "Find places by name or address (Nominatim)",
		Example: `  osmctl geocode "Diyarbakır Surları"
  osmctl geocode "Gazi Caddesi, Diyarbakır" --country tr -n 3
  osmctl geocode "Hevsel Gardens" -o geojson`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ps, err := a.nominatim().Search(cmd.Context(), strings.Join(args, " "), opts)
			if err != nil {
				return err
			}
			return output.Places(a.out, ps, a.format)
		},
	}
	f := cmd.Flags()
	f.IntVarP(&opts.Limit, "limit", "n", 5, "maximum number of results (max 40)")
	f.StringSliceVarP(&opts.Countries, "country", "c", nil, "limit to ISO 3166-1 country codes, e.g. tr,iq")
	f.StringVarP(&opts.Language, "lang", "l", "", "preferred result language, e.g. ku, tr, en")
	return cmd
}

func newReverseCmd(a *app) *cobra.Command {
	var (
		zoom int
		lang string
	)
	cmd := &cobra.Command{
		Use:     "reverse <lat,lon>",
		Short:   "Find the address at a point (Nominatim)",
		Example: "  osmctl reverse 37.9144,40.2306\n  osmctl reverse 37.9144,40.2306 --zoom 10",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := geo.ParsePoint(args[0])
			if err != nil {
				return err
			}
			place, err := a.nominatim().Reverse(cmd.Context(), p.Lat, p.Lon, zoom, lang)
			if err != nil {
				return err
			}
			return output.Places(a.out, []nominatim.Place{*place}, a.format)
		},
	}
	cmd.Flags().IntVarP(&zoom, "zoom", "z", 18, "detail level from 3 (country) to 18 (building)")
	cmd.Flags().StringVarP(&lang, "lang", "l", "", "preferred result language, e.g. ku, tr, en")
	return cmd
}
