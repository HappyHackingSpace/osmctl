package cli

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/happyhackingspace/osmctl/internal/geo"
	"github.com/happyhackingspace/osmctl/internal/osm"
	"github.com/happyhackingspace/osmctl/internal/output"
	"github.com/happyhackingspace/osmctl/internal/stage"
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
		Short: "Create new nodes and ways",
		Long: `Stage new elements. They get negative placeholder ids (n-1, w-2, ...)
that you can use in later commands; the server assigns real ids on push.`,
	}

	var at string
	node := &cobra.Command{
		Use:     "node --at <lat,lon> [key=value]...",
		Short:   "Add a point, such as a shop, bench or tree",
		Example: `  osmctl add node --at 37.9144,40.2306 amenity=bench backrest=yes`,
		RunE: func(_ *cobra.Command, args []string) error {
			p, err := geo.ParsePoint(at)
			if err != nil {
				return err
			}
			tags, err := stage.ParseTags(args)
			if err != nil {
				return err
			}
			return a.edit(func(s *stage.Stage) (string, error) {
				return "Added " + s.AddNode(p.Lat, p.Lon, tags).String(), nil
			})
		},
	}
	node.Flags().StringVar(&at, "at", "", "location as lat,lon")
	_ = node.MarkFlagRequired("at")

	var (
		nodes  []string
		closed bool
	)
	way := &cobra.Command{
		Use:   "way --nodes <n1,n2,...> [key=value]...",
		Short: "Add a line or area through existing or staged nodes",
		Example: `  osmctl add node --at 37.91,40.23
  osmctl add node --at 37.92,40.24
  osmctl add way --nodes n-1,n-2,n123456 highway=footway`,
		RunE: func(_ *cobra.Command, args []string) error {
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
			tags, err := stage.ParseTags(args)
			if err != nil {
				return err
			}
			return a.edit(func(s *stage.Stage) (string, error) {
				id, err := s.AddWay(ids, tags)
				return "Added " + id.String(), err
			})
		},
	}
	way.Flags().StringSliceVar(&nodes, "nodes", nil, "node ids in order, e.g. n-1,n-2,n123")
	way.Flags().BoolVar(&closed, "close", false, "close the way into a ring (for areas)")
	_ = way.MarkFlagRequired("nodes")

	cmd.AddCommand(node, way)
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
			var c osm.Change
			if err := xml.NewDecoder(r).Decode(&c); err != nil {
				return fmt.Errorf("read osmChange: %w", err)
			}
			return a.edit(func(s *stage.Stage) (string, error) {
				before := s.Len()
				if err := s.Apply(&c); err != nil {
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
