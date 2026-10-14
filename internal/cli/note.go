package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/happyhackingspace/osmctl/internal/api"
	"github.com/happyhackingspace/osmctl/internal/auth"
	"github.com/happyhackingspace/osmctl/internal/geo"
	"github.com/happyhackingspace/osmctl/internal/osm"
	"github.com/happyhackingspace/osmctl/internal/output"
)

func newNoteCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Report problems and resolve map notes",
		Long: `Map notes are how anyone reports a problem or a missing place without
editing the map directly. Mappers then fix the map and close the note.`,
	}
	cmd.AddCommand(newNoteListCmd(a), newNoteViewCmd(a), newNoteCreateCmd(a),
		newNoteActionCmd(a, "comment", "Add a comment to a note", true, (*api.Client).CommentNote),
		newNoteActionCmd(a, "close", "Close a resolved note", false, (*api.Client).CloseNote),
		newNoteActionCmd(a, "reopen", "Reopen a closed note", false, (*api.Client).ReopenNote),
	)
	return cmd
}

func parseNoteID(s string) (int64, error) {
	n, err := strconv.ParseInt(strings.TrimPrefix(s, "note/"), 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("note id must be a positive number: %q", s)
	}
	return n, nil
}

func newNoteListCmd(a *app) *cobra.Command {
	var (
		q            api.NoteQuery
		near, bbox   string
		radius       float64
		all, closedF bool
	)
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"search"},
		Short:   "List notes in an area or matching text",
		Example: `  osmctl note list --near 37.9144,40.2306 --radius 2000
  osmctl note list --bbox 40.1,37.8,40.3,38.0 --all
  osmctl note list --text "missing road"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			switch {
			case near != "":
				p, err := geo.ParsePoint(near)
				if err != nil {
					return err
				}
				q.Bounds = geo.Around(p, radius)
			case bbox != "":
				b, err := geo.ParseBBox(bbox)
				if err != nil {
					return err
				}
				q.Bounds = b
			case q.Text == "":
				return errors.New("give an area (--near or --bbox) or --text")
			}
			switch {
			case all:
				q.ClosedDays = -1
			case closedF:
				q.ClosedDays = 7
			}
			ns, err := a.client().Notes(cmd.Context(), q)
			if err != nil {
				return err
			}
			return output.Notes(a.out, ns, a.format)
		},
	}
	f := cmd.Flags()
	f.StringVar(&near, "near", "", "notes around lat,lon")
	f.Float64VarP(&radius, "radius", "r", 1000, "radius in meters for --near")
	f.StringVar(&bbox, "bbox", "", "notes in minlon,minlat,maxlon,maxlat")
	f.StringVar(&q.Text, "text", "", "only notes containing this text")
	f.BoolVar(&all, "all", false, "include closed notes of any age")
	f.BoolVar(&closedF, "recently-closed", false, "include notes closed in the last 7 days")
	f.IntVarP(&q.Limit, "limit", "n", 100, "maximum number of notes (max 10000)")
	cmd.MarkFlagsMutuallyExclusive("near", "bbox")
	cmd.MarkFlagsMutuallyExclusive("all", "recently-closed")
	return cmd
}

func newNoteViewCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "view <id>",
		Short:   "Show a note and its conversation",
		Example: "  osmctl note view 4123456",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseNoteID(args[0])
			if err != nil {
				return err
			}
			n, err := a.client().Note(cmd.Context(), id)
			if err != nil {
				return notFoundHint(err, fmt.Sprintf("note/%d", id))
			}
			return output.Note(a.out, n, a.format)
		},
	}
}

func newNoteCreateCmd(a *app) *cobra.Command {
	var at, text string
	cmd := &cobra.Command{
		Use:   "create --at <lat,lon> -m <text>",
		Short: "Report a problem or missing detail at a location",
		Long: `Create a map note. Describe what is wrong or missing so that someone on
the ground can fix it. Logged-in notes are attributed to you; without a
login the note is created anonymously.`,
		Example: `  osmctl note create --at 37.9144,40.2306 -m "The bakery here closed in 2026; now a pharmacy."`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := geo.ParsePoint(at)
			if err != nil {
				return err
			}
			c, err := a.authClient()
			if errors.Is(err, auth.ErrNotLoggedIn) {
				fmt.Fprintln(a.errOut, "Not logged in: creating the note anonymously.")
				c, err = a.client(), nil
			}
			if err != nil {
				return err
			}
			n, err := c.CreateNote(cmd.Context(), p.Lat, p.Lon, text)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.errOut, "Created %s/note/%d\n", a.srv.Web, n.ID)
			return output.Note(a.out, n, a.format)
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "location as lat,lon")
	cmd.Flags().StringVarP(&text, "message", "m", "", "what is wrong or missing")
	_ = cmd.MarkFlagRequired("at")
	_ = cmd.MarkFlagRequired("message")
	return cmd
}

type noteAction func(*api.Client, context.Context, int64, string) (*osm.Note, error)

func newNoteActionCmd(a *app, verb, short string, textRequired bool, action noteAction) *cobra.Command {
	var text string
	cmd := &cobra.Command{
		Use:     verb + " <id>",
		Short:   short,
		Example: fmt.Sprintf(`  osmctl note %s 4123456 -m "Fixed in changeset 151234567, thanks!"`, verb),
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseNoteID(args[0])
			if err != nil {
				return err
			}
			c, err := a.authClient()
			if err != nil {
				return err
			}
			n, err := action(c, cmd.Context(), id, text)
			if err != nil {
				return notFoundHint(err, fmt.Sprintf("note/%d", id))
			}
			return output.Note(a.out, n, a.format)
		},
	}
	cmd.Flags().StringVarP(&text, "message", "m", "", "comment text")
	if textRequired {
		_ = cmd.MarkFlagRequired("message")
	}
	return cmd
}
