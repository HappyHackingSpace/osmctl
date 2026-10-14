# osmctl

**OpenStreetMap from the terminal.** One tool for reading the map, searching it, editing it and contributing back to it.

```sh
osmctl find amenity=drinking_water --in "Diyarbakır"   # search
osmctl tag node/2187164523 opening_hours="Mo-Sa 08:00-20:00"
osmctl diff                                              # review
osmctl push -m "Add opening hours from survey" --source survey   # contribute
```

Other OSM command line tools each cover one job: osmium handles files, Overpass and Nominatim are web APIs, and editing needs JOSM. osmctl puts all of it behind one consistent interface:

| | |
|---|---|
| **Read** | elements, full history, changesets with discussions, users, raw map data |
| **Search** | tag searches near a point, in a box or in a named place; raw Overpass QL; geocoding and reverse geocoding |
| **Edit** | a git-like workflow: stage tags, nodes, ways, moves and deletions, review the diff, check for mistakes, push one atomic changeset |
| **Contribute** | create, comment on, close and reopen map notes; comment on changesets |
| **Work offline** | download regional extracts, inspect `.osm.pbf` files, extract features by tag or area |
| **Integrate** | every command prints `text`, `json`, `xml` or `geojson`, ready for `jq`, QGIS, JOSM or your scripts |

## Install

With Go 1.27 or newer:

```sh
go install github.com/happyhackingspace/osmctl/cmd/osmctl@latest
```

Or download a binary for Linux, macOS or Windows from the [releases page](https://github.com/happyhackingspace/osmctl/releases). Archives also include man pages (`docs/man`) and shell completions for bash, zsh and fish (`completions/`); `osmctl completion --help` shows how to install them.

## A quick tour

```sh
# Read
osmctl get node/1                                  # the first node ever mapped
osmctl get w50637691 --full -o geojson > gate.geojson
osmctl history n1
osmctl changeset list --user "Some Mapper"
osmctl browse w42                                  # open it on openstreetmap.org

# Search
osmctl geocode "Hevsel Gardens"
osmctl reverse 37.9144,40.2306
osmctl find amenity=cafe --near 37.9144,40.2306 --radius 500
osmctl find tourism=museum --in "Berlin" -o geojson > museums.geojson
osmctl query -f my-query.overpassql

# Notes
osmctl note list --near 37.9144,40.2306 --radius 2000
osmctl note create --at 37.9144,40.2306 -m "This bakery closed; it is a pharmacy now."

# Edit (practice on the sandbox first: --server dev)
osmctl auth login
osmctl tag node/123 wheelchair=yes fixme-
osmctl add node --at 37.9144,40.2306 amenity=bench backrest=yes
osmctl status
osmctl diff
osmctl check
osmctl push -m "Add bench and wheelchair access from survey" --source survey

# Offline
osmctl data download turkey
osmctl data info turkey-latest.osm.pbf
osmctl data filter turkey-latest.osm.pbf amenity=drinking_water -o geojson > water.geojson
```

## Documentation

- [Getting started](docs/getting-started.md): elements, servers, output formats
- [Authentication](docs/authentication.md): logging in, tokens, CI, self-hosted instances
- [Searching](docs/searching.md): `find`, `query`, `geocode` and `reverse`
- [Editing](docs/editing.md): the stage, diff and push workflow, conflicts, good practice
- [Notes](docs/notes.md): reporting and resolving problems
- [Local data](docs/local-data.md): extracts, `.osm.pbf` files and conversion
- [Command reference](docs/commands/osmctl.md): every command and flag, generated from the code

Every command also documents itself: `osmctl <command> --help`.

## Configuration

osmctl needs no config file. These environment variables change its defaults:

| Variable | Purpose |
|---|---|
| `OSMCTL_SERVER` | default server: `osm` (production), `dev` (sandbox) or a URL |
| `OSMCTL_TOKEN` | OAuth 2.0 access token, for CI and machines without a keychain |
| `OSMCTL_CLIENT_ID` | OAuth application id for the dev server or a self-hosted instance (openstreetmap.org works out of the box) |
| `OSMCTL_OVERPASS_URL` | another Overpass instance |
| `OSMCTL_NOMINATIM_URL` | another Nominatim instance |
| `OSMCTL_API_URL` | override the API base URL |
| `OSMCTL_STAGE_DIR` | where staged edits are kept |

## Mapping responsibly

OpenStreetMap is edited by people, for people. osmctl makes editing fast, so:

- **Practice on the sandbox.** `--server dev` points every command at the [development server](https://master.apis.dev.openstreetmap.org), where nothing you do affects the real map.
- **Write a useful changeset comment.** Say what you changed and why, and name your `--source`.
- **Bulk and scripted edits need discussion first.** Follow the [Automated Edits code of conduct](https://wiki.openstreetmap.org/wiki/Automated_Edits_code_of_conduct). osmctl warns you before pushing large changesets.
- **Respect the services.** Nominatim and Overpass are run by volunteers. osmctl identifies itself, retries politely and never runs queries in parallel; please don't script heavy loads against the public instances.

## Contributing

Bug reports, ideas and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md). Report security problems privately as described in [SECURITY.md](SECURITY.md).

## License

osmctl is [MIT licensed](LICENSE). Map data is © [OpenStreetMap contributors](https://www.openstreetmap.org/copyright), available under the [Open Database License](https://opendatacommons.org/licenses/odbl/).

Made by [Happy Hacking Space](https://github.com/happyhackingspace).
