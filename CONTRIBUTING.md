# Contributing

Thanks for helping. Issues and pull requests are welcome, from typo fixes to new commands.

## Setup

You need Go 1.27 or newer, and [golangci-lint](https://golangci-lint.run) to lint.

```sh
git clone https://github.com/happyhackingspace/osmctl
cd osmctl
make build    # ./osmctl
make check    # lint, tests, and docs up to date
```

## Layout

```
cmd/osmctl         entry point
internal/cli       commands (cobra); one file per command group
internal/osm       OSM data model and codecs: XML, JSON, osmChange, PBF, GeoJSON
internal/api       OSM API v0.6 client
internal/auth      OAuth 2.0 PKCE login and keychain storage
internal/stage     local staging of edits, diff, osmChange apply
internal/overpass  Overpass client and query builder
internal/nominatim Nominatim client
internal/osmfile   local .osm/.pbf reading and extraction
internal/geofabrik regional extract index and download
internal/tagfilter key=value filter parsing and matching
internal/output    text, json, xml and geojson rendering
internal/browser   opening URLs in the default browser
tools/gendocs      generates docs/commands from the command tree
```

## Guidelines

- **Keep dependencies minimal.** osmctl depends only on the Go standard library, `golang.org/x`, cobra and go-keyring. OSM formats are implemented in `internal/osm`; extend them there rather than adding a library.
- **Test against fakes, not the live services.** Use `httptest` servers; see `internal/cli/cli_test.go` for the edit flow end to end. Never write tests that edit the real map.
- **Manual testing of edits goes to the sandbox:** `--server dev`.
- **Keep commands consistent.** Elements are parsed by `osm.ParseID`, points are `lat,lon`, bounding boxes are `minlon,minlat,maxlon,maxlat`, and every command honors `-o`.
- **Data on stdout, messages on stderr.**
- **Update the docs.** When you change a command, run `make docs` and commit `docs/commands`. CI fails if they drift. Update the guides in `docs/` when behavior changes.
- **Comments explain why, not what.**
- **One topic per pull request**, with a short description and `Fixes #N` when there is an issue.

## Reporting bugs

Include the command you ran, what you expected, what happened, and `osmctl --version`. Never paste a token: `OSMCTL_TOKEN` values and keychain contents are secrets.
