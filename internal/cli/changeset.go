package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/happyhackingspace/osmctl/internal/api"
	"github.com/happyhackingspace/osmctl/internal/geo"
	"github.com/happyhackingspace/osmctl/internal/output"
)

func newChangesetCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "changeset",
		Aliases: []string{"cs"},
		Short:   "Inspect, list, download and discuss changesets",
	}
	cmd.AddCommand(newChangesetViewCmd(a), newChangesetListCmd(a), newChangesetDownloadCmd(a), newChangesetCommentCmd(a))
	return cmd
}

func parseChangesetID(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("changeset id must be a positive number: %q", s)
	}
	return n, nil
}

func newChangesetViewCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "view <id>",
		Short:   "Show a changeset with its tags and discussion",
		Example: "  osmctl changeset view 151234567",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseChangesetID(args[0])
			if err != nil {
				return err
			}
			c, err := a.client().Changeset(cmd.Context(), id)
			if err != nil {
				return notFoundHint(err, fmt.Sprintf("changeset/%d", id))
			}
			return output.Changeset(a.out, c, a.format)
		},
	}
}

func newChangesetListCmd(a *app) *cobra.Command {
	var (
		q    api.ChangesetQuery
		bbox string
		mine bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent changesets, filtered by user, area or state",
		Example: `  osmctl changeset list --user "Some Mapper"
  osmctl changeset list --mine --open
  osmctl changeset list --bbox 40.1,37.8,40.3,38.0 --limit 20`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if bbox != "" {
				b, err := geo.ParseBBox(bbox)
				if err != nil {
					return err
				}
				q.Bounds = b
			}
			c := a.client()
			if mine {
				ac, err := a.authClient()
				if err != nil {
					return err
				}
				me, err := ac.Me(cmd.Context())
				if err != nil {
					return err
				}
				q.User = strconv.FormatInt(me.ID, 10)
			}
			cs, err := c.Changesets(cmd.Context(), q)
			if err != nil {
				return err
			}
			return output.Changesets(a.out, cs, a.format)
		},
	}
	f := cmd.Flags()
	f.StringVarP(&q.User, "user", "u", "", "user display name or numeric id")
	f.BoolVar(&mine, "mine", false, "only your changesets (requires login)")
	f.StringVar(&bbox, "bbox", "", "only changesets intersecting minlon,minlat,maxlon,maxlat")
	f.BoolVar(&q.Open, "open", false, "only open changesets")
	f.BoolVar(&q.Closed, "closed", false, "only closed changesets")
	f.IntVarP(&q.Limit, "limit", "n", 25, "maximum number of changesets (API maximum is 100)")
	cmd.MarkFlagsMutuallyExclusive("user", "mine")
	cmd.MarkFlagsMutuallyExclusive("open", "closed")
	return cmd
}

func newChangesetDownloadCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "download <id>",
		Short: "Download what a changeset changed, as osmChange",
		Long: `Download the full content of a changeset as osmChange.

The default text output summarizes it; use -o xml for a .osc file that
` + "`osmctl apply`" + ` and other tools can read.`,
		Example: "  osmctl changeset download 151234567\n  osmctl changeset download 151234567 -o xml > cs.osc",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseChangesetID(args[0])
			if err != nil {
				return err
			}
			ch, err := a.client().ChangesetDownload(cmd.Context(), id)
			if err != nil {
				return notFoundHint(err, fmt.Sprintf("changeset/%d", id))
			}
			return output.Change(a.out, ch, a.format)
		},
	}
}

func newChangesetCommentCmd(a *app) *cobra.Command {
	var text string
	cmd := &cobra.Command{
		Use:     "comment <id> -m <text>",
		Short:   "Comment on a changeset's discussion",
		Example: `  osmctl changeset comment 151234567 -m "Thanks! Could you add a source tag?"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseChangesetID(args[0])
			if err != nil {
				return err
			}
			c, err := a.authClient()
			if err != nil {
				return err
			}
			if err := c.CommentChangeset(cmd.Context(), id, text); err != nil {
				return err
			}
			_, err = fmt.Fprintf(a.errOut, "Commented on %s/changeset/%d\n", a.srv.Web, id)
			return err
		},
	}
	cmd.Flags().StringVarP(&text, "message", "m", "", "comment text")
	_ = cmd.MarkFlagRequired("message")
	return cmd
}
