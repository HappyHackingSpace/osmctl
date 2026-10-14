# Getting started

## The map in three element types

All OpenStreetMap data is made of three kinds of elements, each carrying **tags** (`key=value` pairs such as `amenity=cafe` or `name=Kafe Amed`):

- **node**: a point with a latitude and longitude, such as a bench, a shop or a corner of a building
- **way**: an ordered list of nodes, forming a line (a road) or, when closed, an area (a building)
- **relation**: a group of elements with roles, such as a bus route or a city boundary

## Naming elements

Every command that takes an element accepts the same forms:

| Form | Example |
|---|---|
| type/id | `node/2187164523`, `way/42`, `relation/7` |
| short | `n2187164523`, `w42`, `r7` |
| URL | `https://www.openstreetmap.org/way/42` |
| staged placeholder | `n-1`, `w-3` (new elements not yet pushed; see [Editing](editing.md)) |

Pasting the URL from your browser always works.

## Coordinates

Points are written `lat,lon`, the order map URLs use:

```sh
osmctl reverse 37.9144,40.2306
```

Bounding boxes are written `minlon,minlat,maxlon,maxlat`, the order the OSM API and most GIS tools use:

```sh
osmctl map --bbox 40.2290,37.9100,40.2330,37.9130
```

Tip: on [openstreetmap.org](https://www.openstreetmap.org), choose **Export** and then **Manually select a different area** to read off a bounding box.

## Output formats

Every command takes `-o` (`--output`):

| Format | Use it for |
|---|---|
| `text` | reading in the terminal (default) |
| `json` | scripts and `jq` |
| `xml` | OSM XML files for JOSM and other OSM tools |
| `geojson` | QGIS, uMap, Leaflet, geojson.io and most GIS software |

```sh
osmctl get w42 -o json | jq '.elements[0].tags'
osmctl find leisure=park --in "Amed" -o geojson > parks.geojson
```

Data goes to standard output and messages go to standard error, so redirecting the output never mixes in status messages.

## Servers

osmctl talks to the main OpenStreetMap by default. `--server` (or `OSMCTL_SERVER`) changes that:

| Server | Meaning |
|---|---|
| `osm` | openstreetmap.org (default) |
| `dev` | the [development sandbox](https://master.apis.dev.openstreetmap.org): a separate database for testing edits |
| a URL | a self-hosted instance running the OSM website software |

```sh
osmctl --server dev get node/1
export OSMCTL_SERVER=dev    # practice mode for the whole shell session
```

Logins and staged edits are kept separately for each server, so practice edits can never be pushed to the real map by accident.

## Next steps

- [Searching](searching.md) to find things on the map
- [Editing](editing.md) to change the map
- [Notes](notes.md) to report problems without editing
- [Local data](local-data.md) to work with files offline
