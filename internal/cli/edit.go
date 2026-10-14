package cli

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/happyhackingspace/osmctl/internal/geo"
	"github.com/happyhackingspace/osmctl/internal/gpx"
	"github.com/happyhackingspace/osmctl/internal/osm"
	"github.com/happyhackingspace/osmctl/internal/output"
	"github.com/happyhackingspace/osmctl/internal/stage"
	"github.com/happyhackingspace/osmctl/internal/validate"
)

// edit loads the stage for the current server, applies fn and saves it.
func (a *app) edit(fn func(*stage.Stage) (string, error)) error {
	s, err := stage.Load(a.srv.Web)
	if err != nil {
		return err
	}
	what, err := fn(s)
	if err != nil {
		return err
	}
	if err := s.Save(); err != nil {
		return err
	}
	if s.Empty() {
		_, err = fmt.Fprintf(a.errOut, "%s. Nothing staged for %s.\n", what, a.srv.Web)
		return err
	}
	_, err = fmt.Fprintf(a.errOut, "%s. %d change(s) staged for %s; review with `osmctl diff`, upload with `osmctl push`.\n", what, s.Len(), a.srv.Web)
	return err
}

func newTagCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "tag <element> <key=value | key->...",
		Short: "Set or remove tags on an element",
		Long: `Stage tag changes on an element: key=value sets a tag, key- removes it.

Quote values with spaces. Nothing is uploaded until ` + "`osmctl push`" + `.`,
		Example: `  osmctl tag node/2187164523 opening_hours="Mo-Sa 08:00-20:00" wheelchair=yes
  osmctl tag w42 name:ku="Kela Amedê" fixme-`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := osm.ParseID(args[0])
			if err != nil {
				return err
			}
			e, err := stage.ParseTagEdits(args[1:])
			if err != nil {
				return err
			}
			return a.edit(func(s *stage.Stage) (string, error) {
				return "Tagged " + id.String(), s.Tag(cmd.Context(), a.client(), id, e)
			})
		},
	}
}

func newAddCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create new nodes, ways and relations",
		Long: `Stage new elements. They get negative placeholder ids (n-1, w-2, ...)
that you can use in later commands; the server assigns real ids on push.`,
	}

	var (
		at string
		on []string
	)
	node := &cobra.Command{
		Use:   "node --at <lat,lon> [--on <way>]... [key=value]...",
		Short: "Add a point, or a node on existing ways to connect to them",
		Long: `Add a node at a location.

With --on, the node is placed onto the nearest point of that way instead,
so new roads can be joined to it. Repeat --on to make a junction shared by
several ways. If a node of the way is already within a metre, that node is
used instead of adding a new one next to it.`,
		Example: `  osmctl add node --at 37.9144,40.2306 amenity=bench backrest=yes
  osmctl add node --at 38.43412,40.28429 --on way/362907199
  osmctl add node --at 38.4401,40.2853 --on w1568227783 --on w362907199`,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := geo.ParsePoint(at)
			if err != nil {
				return err
			}
			tags, err := stage.ParseTags(args)
			if err != nil {
				return err
			}
			ways := make([]osm.ID, 0, len(on))
			for _, ref := range on {
				id, err := osm.ParseID(ref)
				if err != nil {
					return err
				}
				ways = append(ways, id)
			}
			return a.edit(func(s *stage.Stage) (string, error) {
				if len(ways) == 0 {
					return "Added " + s.AddNode(p.Lat, p.Lon, tags).String(), nil
				}
				id, created, err := s.AddNodeOnWays(cmd.Context(), a.client(), ways, p.Lat, p.Lon, tags)
				if err != nil {
					return "", err
				}
				verb := "Added"
				if !created {
					verb = "Using existing"
				}
				return fmt.Sprintf("%s %s on %s (use it in add way --nodes)", verb, id, strings.Join(on, ", ")), nil
			})
		},
	}
	node.Flags().StringVar(&at, "at", "", "location as lat,lon")
	node.Flags().StringSliceVar(&on, "on", nil, "place the node onto this way (repeatable)")
	_ = node.MarkFlagRequired("at")

	var (
		nodes  []string
		closed bool
	)
	var (
		gpxFile, startOn, endOn string
		track                   int
		tolerance               float64
	)
	way := &cobra.Command{
		Use:   "way (--nodes <n1,n2,...> | --from-gpx <file>) [key=value]...",
		Short: "Add a line or area through nodes, or from a GPS track",
		Long: `Add a way through existing or staged nodes, or from a recorded GPS track.

With --from-gpx, the track is simplified (Douglas-Peucker, --simplify metres)
so GPS jitter does not become hundreds of nodes. --start-on and --end-on
place the first and last node onto existing ways, so the new road is
connected to the network.`,
		Example: `  osmctl add node --at 37.91,40.23
  osmctl add node --at 37.92,40.24 --on way/123456
  osmctl add way --nodes n-1,n-2 highway=footway
  osmctl add way --from-gpx drive.gpx --start-on way/362907199 highway=track surface=unpaved`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tags, err := stage.ParseTags(args)
			if err != nil {
				return err
			}
			if gpxFile != "" {
				return a.addWayFromGPX(cmd.Context(), gpxFile, track, tolerance, startOn, endOn, tags)
			}
			ids := make([]int64, 0, len(nodes)+1)
			for _, ref := range nodes {
				id, err := parseNodeRef(ref)
				if err != nil {
					return err
				}
				ids = append(ids, id)
			}
			if closed && len(ids) > 0 && ids[0] != ids[len(ids)-1] {
				ids = append(ids, ids[0])
			}
			return a.edit(func(s *stage.Stage) (string, error) {
				id, err := s.AddWay(ids, tags)
				return "Added " + id.String(), err
			})
		},
	}
	wf := way.Flags()
	wf.StringSliceVar(&nodes, "nodes", nil, "node ids in order, e.g. n-1,n-2,n123")
	wf.BoolVar(&closed, "close", false, "close the way into a ring (for areas)")
	wf.StringVar(&gpxFile, "from-gpx", "", "build the way from a GPX track")
	wf.IntVar(&track, "track", 1, "which track or route in the GPX file (1 is the first)")
	wf.Float64Var(&tolerance, "simplify", 3, "with --from-gpx: drop points within this many metres of the line")
	wf.StringVar(&startOn, "start-on", "", "with --from-gpx: place the first node onto this way")
	wf.StringVar(&endOn, "end-on", "", "with --from-gpx: place the last node onto this way")
	way.MarkFlagsOneRequired("nodes", "from-gpx")
	way.MarkFlagsMutuallyExclusive("nodes", "from-gpx")
	way.MarkFlagsMutuallyExclusive("close", "from-gpx")

	var members []string
	relation := &cobra.Command{
		Use:   "relation --member <element[:role]>... [key=value]...",
		Short: "Add a relation, such as a route or a multipolygon",
		Example: `  osmctl add relation type=route route=hiking name="Pîr Ezîz trail" --member w1568227783 --member w-3
  osmctl add relation type=multipolygon natural=water --member w-1:outer --member w-2:inner`,
		RunE: func(_ *cobra.Command, args []string) error {
			ms, err := parseMembers(members)
			if err != nil {
				return err
			}
			tags, err := stage.ParseTags(args)
			if err != nil {
				return err
			}
			return a.edit(func(s *stage.Stage) (string, error) {
				id, err := s.AddRelation(ms, tags)
				return "Added " + id.String(), err
			})
		},
	}
	relation.Flags().StringSliceVar(&members, "member", nil, "member as element or element:role, in order (repeatable)")
	_ = relation.MarkFlagRequired("member")

	cmd.AddCommand(node, way, relation)
	return cmd
}

func (a *app) addWayFromGPX(ctx context.Context, file string, track int, tolerance float64, startOn, endOn string, tags osm.Tags) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	tracks, err := gpx.Read(f)
	_ = f.Close()
	if err != nil {
		return err
	}
	if track < 1 || track > len(tracks) {
		return fmt.Errorf("%s has %d track(s); --track must be between 1 and %d", file, len(tracks), len(tracks))
	}
	t := tracks[track-1]
	pts := gpx.Simplify(t.Points, tolerance)
	if len(pts) < 2 {
		return fmt.Errorf("track %d has fewer than two points", track)
	}
	if len(pts) > stage.MaxWayNodes {
		return fmt.Errorf("the simplified track still has %d points, more than a way may have (%d); raise --simplify", len(pts), stage.MaxWayNodes)
	}
	ends := map[int]string{0: startOn, len(pts) - 1: endOn}
	return a.edit(func(s *stage.Stage) (string, error) {
		ids := make([]int64, 0, len(pts))
		for i, p := range pts {
			if on := ends[i]; on != "" {
				w, err := osm.ParseID(on)
				if err != nil {
					return "", err
				}
				id, _, err := s.AddNodeOnWays(ctx, a.client(), []osm.ID{w}, p.Lat, p.Lon, nil)
				if err != nil {
					return "", err
				}
				ids = append(ids, id.Ref)
				continue
			}
			ids = append(ids, s.AddNode(p.Lat, p.Lon, nil).Ref)
		}
		id, err := s.AddWay(ids, tags)
		return fmt.Sprintf("Added %s with %d nodes from %d GPS points", id, len(ids), len(t.Points)), err
	})
}

func parseMembers(args []string) ([]osm.Member, error) {
	out := make([]osm.Member, 0, len(args))
	for _, a := range args {
		m, err := stage.ParseMember(a)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func newMemberCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "member",
		Short: "Add or remove members of a relation",
		Long:  "Change which elements belong to a relation. Members are written as element or\nelement:role, for example way/12:outer or n-1.",
	}
	var at int
	add := &cobra.Command{
		Use:   "add <relation> <element[:role]>...",
		Short: "Add members to a relation",
		Example: `  osmctl member add r1234 way/5678:outer
  osmctl member add r1234 n-1:stop --at 0`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := osm.ParseID(args[0])
			if err != nil {
				return err
			}
			ms, err := parseMembers(args[1:])
			if err != nil {
				return err
			}
			pos := -1
			if cmd.Flags().Changed("at") {
				pos = at
			}
			return a.edit(func(s *stage.Stage) (string, error) {
				return fmt.Sprintf("Added %d member(s) to %s", len(ms), id), s.AddMembers(cmd.Context(), a.client(), id, ms, pos)
			})
		},
	}
	add.Flags().IntVar(&at, "at", 0, "insert at this position (0 is first; default is the end)")

	remove := &cobra.Command{
		Use:     "remove <relation> <element>...",
		Short:   "Remove members from a relation",
		Example: "  osmctl member remove r1234 way/5678",
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := osm.ParseID(args[0])
			if err != nil {
				return err
			}
			elements := make([]osm.ID, 0, len(args)-1)
			for _, arg := range args[1:] {
				e, err := osm.ParseID(arg)
				if err != nil {
					return err
				}
				elements = append(elements, e)
			}
			return a.edit(func(s *stage.Stage) (string, error) {
				return fmt.Sprintf("Removed %d member(s) from %s", len(elements), id), s.RemoveMembers(cmd.Context(), a.client(), id, elements)
			})
		},
	}
	cmd.AddCommand(add, remove)
	return cmd
}

func parseNodeRef(s string) (int64, error) {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n != 0 {
		return n, nil
	}
	id, err := osm.ParseID(s)
	if err != nil {
		return 0, err
	}
	if id.Type != osm.TypeNode {
		return 0, fmt.Errorf("%s is not a node", id)
	}
	return id.Ref, nil
}

func newMoveCmd(a *app) *cobra.Command {
	var to string
	cmd := &cobra.Command{
		Use:     "move <node> --to <lat,lon>",
		Short:   "Move a node to a new location",
		Example: "  osmctl move node/2187164523 --to 37.91452,40.23071",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := osm.ParseID(args[0])
			if err != nil {
				return err
			}
			p, err := geo.ParsePoint(to)
			if err != nil {
				return err
			}
			return a.edit(func(s *stage.Stage) (string, error) {
				return "Moved " + id.String(), s.Move(cmd.Context(), a.client(), id, p.Lat, p.Lon)
			})
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "new location as lat,lon")
	_ = cmd.MarkFlagRequired("to")
	return cmd
}

func newDeleteCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <element>...",
		Short: "Delete elements",
		Long: `Stage elements for deletion. The server refuses to delete a node that a
way still uses, or an element that a relation still lists; delete or edit
those first, in the same push.`,
		Example: "  osmctl delete node/2187164523\n  osmctl delete w42 n100 n101",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := make([]osm.ID, 0, len(args))
			for _, arg := range args {
				id, err := osm.ParseID(arg)
				if err != nil {
					return err
				}
				ids = append(ids, id)
			}
			return a.edit(func(s *stage.Stage) (string, error) {
				for _, id := range ids {
					if err := s.Remove(cmd.Context(), a.client(), id); err != nil {
						return "", err
					}
				}
				return fmt.Sprintf("Deleted %d element(s)", len(ids)), nil
			})
		},
	}
}

func newApplyCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "apply <file.osc | ->",
		Short: "Stage an osmChange file made by another tool",
		Long: `Stage every change in an osmChange (.osc) file, for example one written by
a script or exported from JOSM. Use - to read standard input.

Modified and deleted elements must carry the version they were based on,
so the server can detect conflicts.`,
		Example: "  osmctl apply fixes.osc\n  python fix_names.py | osmctl apply -",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			r := a.in
			if args[0] != "-" {
				f, err := os.Open(args[0])
				if err != nil {
					return err
				}
				defer func() { _ = f.Close() }()
				r = f
			}
			c, err := osm.ReadChange(r)
			if err != nil {
				return err
			}
			return a.edit(func(s *stage.Stage) (string, error) {
				before := s.Len()
				if err := s.Apply(c); err != nil {
					return "", err
				}
				return fmt.Sprintf("Applied %d change(s)", s.Len()-before), nil
			})
		},
	}
}

func newStatusCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "List staged changes",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			s, err := stage.Load(a.srv.Web)
			if err != nil {
				return err
			}
			return output.Status(a.out, s.List(), a.format)
		},
	}
}

func newDiffCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "diff",
		Short: "Show staged changes in detail",
		Long: `Show what every staged change does: tags added (+), removed (-) and
changed (~), nodes moved, elements created and deleted.

-o xml prints the osmChange that push uploads.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := stage.Load(a.srv.Web)
			if err != nil {
				return err
			}
			if a.format == output.XML {
				b, err := osm.EncodeUpload(s.Change(), 0)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(a.out, "%s%s\n", xml.Header, b)
				return err
			}
			entries, err := s.Diff(cmd.Context(), a.client())
			if err != nil {
				return err
			}
			// Diff may have cached base versions; keep them for next time.
			if err := s.Save(); err != nil {
				return err
			}
			return output.Diff(a.out, entries, a.format)
		},
	}
}

func newCheckCmd(a *app) *cobra.Command {
	var comment string
	var offline bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Look for mistakes in staged changes",
		Long: `Check staged changes the way iD and JOSM do before saving.

Errors would break the data and stop ` + "`osmctl push`" + `: untagged nodes that belong
to nothing, references to deleted elements, malformed ways.

Warnings deserve a look: roads not connected to the road network, roads
crossing other roads, railways or waterways without a junction, deprecated
or vague tags, and (with -m) a vague changeset comment.

The connection and crossing checks download the map around the edits;
--offline skips them.`,
		Example: "  osmctl check\n  osmctl check -m \"Add footpath to the shrine\"",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := stage.Load(a.srv.Web)
			if err != nil {
				return err
			}
			if s.Empty() {
				return errors.New("nothing staged")
			}
			issues := a.validate(cmd.Context(), s, offline)
			if cmd.Flags().Changed("message") {
				issues = append(issues, validate.Comment(comment)...)
			}
			if err := output.Issues(a.out, issues, a.format); err != nil {
				return err
			}
			if validate.HasErrors(issues) {
				return errors.New("staged changes have errors")
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&comment, "message", "m", "", "also check this changeset comment")
	cmd.Flags().BoolVar(&offline, "offline", false, "skip the checks that download surrounding map data")
	return cmd
}

func (a *app) validate(ctx context.Context, s *stage.Stage, offline bool) []validate.Issue {
	var src validate.MapSource
	if !offline {
		src = a.client()
	}
	return validate.Check(ctx, s, src)
}

func newResetCmd(a *app) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:     "reset (<element>... | --all)",
		Short:   "Unstage changes",
		Example: "  osmctl reset node/2187164523\n  osmctl reset --all",
		RunE: func(_ *cobra.Command, args []string) error {
			if all == (len(args) > 0) {
				return errors.New("name elements to unstage, or use --all")
			}
			return a.edit(func(s *stage.Stage) (string, error) {
				if all {
					s.Clear()
					return "Unstaged everything", nil
				}
				for _, arg := range args {
					id, err := osm.ParseID(arg)
					if err != nil {
						return "", err
					}
					if err := s.Reset(id); err != nil {
						return "", err
					}
				}
				return "Unstaged " + strings.Join(args, ", "), nil
			})
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "unstage everything")
	return cmd
}
