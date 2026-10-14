// Package cli defines the osmctl command tree.
package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/happyhackingspace/osmctl/internal/api"
	"github.com/happyhackingspace/osmctl/internal/auth"
	"github.com/happyhackingspace/osmctl/internal/httpx"
	"github.com/happyhackingspace/osmctl/internal/output"
	"github.com/happyhackingspace/osmctl/internal/server"
)

// app holds what every command needs: the chosen server, output format and I/O.
type app struct {
	version    string
	serverFlag string
	formatFlag string

	srv    server.Server
	format output.Format
	store  auth.Store
	in     io.Reader
	out    io.Writer
	errOut io.Writer
}

func (a *app) http() *http.Client {
	return httpx.New(a.version, nil, 5*time.Minute)
}

func (a *app) withToken(token string) *api.Client {
	return api.New(a.srv.API, httpx.New(a.version, auth.Transport(token, http.DefaultTransport), 5*time.Minute))
}

// client is for reads, which OSM serves anonymously. It deliberately skips the
// keychain: touching it can raise an OS access prompt on every rebuild.
func (a *app) client() *api.Client {
	return api.New(a.srv.API, a.http())
}

// authClient is for writes: it fails up front instead of at the first 401.
func (a *app) authClient() (*api.Client, error) {
	tok, _, err := auth.Token(a.store, a.srv)
	if err != nil {
		return nil, err
	}
	return a.withToken(tok), nil
}

func Execute(ctx context.Context, version string) int {
	a := &app{version: version, store: auth.Keyring{}, in: os.Stdin, out: os.Stdout, errOut: os.Stderr}
	root := newRoot(a)
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(a.errOut, "error:", err)
		return 1
	}
	return 0
}

func newRoot(a *app) *cobra.Command {
	defaultServer := server.Production
	if v := os.Getenv("OSMCTL_SERVER"); v != "" {
		defaultServer = v
	}

	root := &cobra.Command{
		Use:   "osmctl",
		Short: "Read, query, edit and contribute to OpenStreetMap from the terminal",
		Long: `osmctl is an all-in-one command line tool for OpenStreetMap.

Read elements, history and changesets, query with Overpass, geocode with
Nominatim, report and resolve notes, edit the map through a git-like
stage, diff and push workflow, and work with local .osm and .pbf files.

Reading needs no account. Editing needs ` + "`osmctl auth login`" + `.
Practice on the sandbox first: ` + "`osmctl --server dev ...`",
		Version:       a.version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			srv, err := server.Lookup(a.serverFlag)
			if err != nil {
				return err
			}
			a.srv = srv
			a.format, err = output.ParseFormat(a.formatFlag)
			return err
		},
	}
	root.SetIn(a.in)
	root.SetOut(a.out)
	root.SetErr(a.errOut)

	pf := root.PersistentFlags()
	pf.StringVarP(&a.serverFlag, "server", "s", defaultServer,
		"OSM instance: "+strings.Join(server.Names(), ", ")+", or a URL (env OSMCTL_SERVER)")
	pf.StringVarP(&a.formatFlag, "output", "o", string(output.Text), "output format: text, json, xml, geojson")

	root.AddGroup(
		&cobra.Group{ID: "read", Title: "Read the map:"},
		&cobra.Group{ID: "search", Title: "Search and query:"},
		&cobra.Group{ID: "edit", Title: "Edit the map:"},
		&cobra.Group{ID: "contribute", Title: "Notes and discussion:"},
		&cobra.Group{ID: "local", Title: "Local files:"},
		&cobra.Group{ID: "account", Title: "Account:"},
	)
	add := func(group string, cmds ...*cobra.Command) {
		for _, c := range cmds {
			c.GroupID = group
			root.AddCommand(c)
		}
	}
	add("read", newGetCmd(a), newHistoryCmd(a), newMapCmd(a), newChangesetCmd(a), newUserCmd(a), newBrowseCmd(a))
	add("search", newQueryCmd(a), newFindCmd(a), newGeocodeCmd(a), newReverseCmd(a))
	add("edit", newTagCmd(a), newAddCmd(a), newMoveCmd(a), newDeleteCmd(a), newApplyCmd(a), newStatusCmd(a), newDiffCmd(a), newResetCmd(a), newPushCmd(a))
	add("contribute", newNoteCmd(a))
	add("local", newDataCmd(a))
	add("account", newAuthCmd(a))
	return root
}
