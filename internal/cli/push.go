package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/happyhackingspace/osmctl/internal/api"
	"github.com/happyhackingspace/osmctl/internal/osm"
	"github.com/happyhackingspace/osmctl/internal/output"
	"github.com/happyhackingspace/osmctl/internal/stage"
	"github.com/happyhackingspace/osmctl/internal/validate"
)

// bulkThreshold is where an upload starts to look like an automated edit,
// which the OSM community expects to be discussed before it happens.
const bulkThreshold = 100

func newPushCmd(a *app) *cobra.Command {
	var (
		comment, source, hashtags string
		extra                     []string
		yes, noCheck              bool
	)
	cmd := &cobra.Command{
		Use:   "push -m <comment>",
		Short: "Upload staged changes as one changeset",
		Long: `Upload every staged change as a single changeset.

The diff and the results of ` + "`osmctl check`" + ` are shown and you are asked to
confirm first; errors stop the push. The upload is atomic:
if any element changed on the server since you staged it, nothing is
written and your stage is kept so you can resolve the conflict.

Write a comment that says what you changed and why; reviewers read it.
Practice on the sandbox with --server dev before editing the real map.`,
		Example: `  osmctl push -m "Add opening hours to cafes on Gazi Caddesi" --source survey
  osmctl --server dev push -m "Testing osmctl" --yes`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if strings.TrimSpace(comment) == "" {
				return errors.New("a changeset comment is required (-m)")
			}
			s, err := stage.Load(a.srv.Web)
			if err != nil {
				return err
			}
			if s.Empty() {
				return errors.New("nothing staged")
			}
			c, err := a.authClient()
			if err != nil {
				return err
			}
			me, err := c.Me(ctx)
			if err != nil {
				return err
			}

			entries, err := s.Diff(ctx, a.client())
			if err != nil {
				return err
			}
			if err := output.Diff(a.errOut, entries, output.Text); err != nil {
				return err
			}
			if !noCheck {
				issues := append(a.validate(ctx, s, false), validate.Comment(comment)...)
				if len(issues) > 0 {
					fmt.Fprintln(a.errOut)
					if err := output.Issues(a.errOut, issues, output.Text); err != nil {
						return err
					}
				}
				if validate.HasErrors(issues) {
					return errors.New("not pushed: fix the errors above, then check again with `osmctl check`")
				}
			}
			if s.Len() >= bulkThreshold {
				fmt.Fprintf(a.errOut, "\nThis is a large edit (%d changes). Automated and mass edits must be discussed with the community first:\n  https://wiki.openstreetmap.org/wiki/Automated_Edits_code_of_conduct\n", s.Len())
			}
			if !yes {
				ok, err := a.confirm(fmt.Sprintf("\nUpload %d change(s) to %s as %s?", s.Len(), a.srv.Web, me.Name))
				if err != nil {
					return err
				}
				if !ok {
					return errors.New("push cancelled")
				}
			}

			tags := osm.Tags{{Key: "comment", Value: comment}, {Key: "created_by", Value: "osmctl " + a.version}}
			if source != "" {
				tags = append(tags, osm.Tag{Key: "source", Value: source})
			}
			if hashtags != "" {
				tags = append(tags, osm.Tag{Key: "hashtags", Value: hashtags})
			}
			more, err := stage.ParseTags(extra)
			if err != nil {
				return err
			}
			tags = append(tags, more...)

			id, res, err := upload(ctx, c, tags, s.Change())
			if err != nil {
				return err
			}
			s.Clear()
			if err := s.Save(); err != nil {
				return fmt.Errorf("pushed changeset %d but could not clear the stage: %w", id, err)
			}
			for _, e := range res.Entries {
				if e.OldID < 0 {
					fmt.Fprintf(a.errOut, "  %s/%d is now %s/%d\n", e.Type(), e.OldID, e.Type(), e.NewID)
				}
			}
			_, err = fmt.Fprintf(a.out, "%s/changeset/%d\n", a.srv.Web, id)
			return err
		},
	}
	f := cmd.Flags()
	f.StringVarP(&comment, "message", "m", "", "changeset comment: what changed and why (required)")
	f.StringVar(&source, "source", "", "where the information came from, e.g. survey, local knowledge, Bing")
	f.StringVar(&hashtags, "hashtags", "", "hashtags for the changeset, e.g. #hhs;#mapathon")
	f.StringSliceVar(&extra, "tag", nil, "extra changeset tag as key=value (repeatable)")
	f.BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation")
	f.BoolVar(&noCheck, "no-check", false, "skip the checks for mistakes (see osmctl check)")
	return cmd
}

// upload opens a changeset, uploads and closes it. The changeset is closed
// even when the upload fails, so no empty changeset lingers open for an hour.
func upload(ctx context.Context, c *api.Client, tags osm.Tags, change *osm.Change) (int64, *api.DiffResult, error) {
	id, err := c.CreateChangeset(ctx, tags)
	if err != nil {
		return 0, nil, fmt.Errorf("open changeset: %w", err)
	}
	res, uerr := c.Upload(ctx, id, change)
	cerr := c.CloseChangeset(context.WithoutCancel(ctx), id)
	if uerr != nil {
		return 0, nil, uploadHint(uerr)
	}
	if cerr != nil {
		return id, res, fmt.Errorf("uploaded to changeset %d but could not close it (it closes itself within an hour): %w", id, cerr)
	}
	return id, res, nil
}

func uploadHint(err error) error {
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		switch apiErr.Status {
		case http.StatusConflict:
			return fmt.Errorf("%s\nSomeone changed an element after you staged it. Nothing was uploaded and your stage is kept. Run `osmctl reset <element>` and redo that edit on the new version", apiErr.Message)
		case http.StatusPreconditionFailed:
			return fmt.Errorf("%s\nAn element you delete is still in use, or one you reference does not exist. Nothing was uploaded and your stage is kept", apiErr.Message)
		}
	}
	return fmt.Errorf("upload failed, nothing was written and your stage is kept: %w", err)
}

func (a *app) confirm(question string) (bool, error) {
	if f, ok := a.in.(*os.File); ok {
		if st, err := f.Stat(); err == nil && st.Mode()&os.ModeCharDevice == 0 {
			return false, errors.New("standard input is not a terminal; pass --yes to confirm")
		}
	}
	fmt.Fprintf(a.errOut, "%s [y/N] ", question)
	line, err := bufio.NewReader(a.in).ReadString('\n')
	if err != nil && line == "" {
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
