# Local data

For larger areas, repeated analysis or offline work, download the data once and work with files. osmctl reads `.osm.pbf`, `.osm`, `.osm.gz` and `.osm.bz2`.

## Getting a regional extract

[Geofabrik](https://download.geofabrik.de) publishes daily extracts of every continent, country and many regions:

```sh
osmctl data regions                 # everything available
osmctl data regions turkey          # search
osmctl data download turkey         # saves turkey-latest.osm.pbf
osmctl data download iraq -O data/iraq.osm.pbf
```

Downloads go to a temporary file first, so an interrupted download never leaves a truncated file.

## Inspecting a file

```sh
osmctl data info turkey-latest.osm.pbf
```

```
nodes       ...
ways        ...
relations   ...
bbox        25.6,35.8,44.8,42.1
edits       2005-11-03 to 2026-10-13
data as of  2026-10-13T20:21:31Z
written by  osmium/1.16.0
```

## Extracting by tag and area

`data filter` uses the same tag filters as [`find`](searching.md#finding-features-by-tag) and works entirely offline:

```sh
osmctl data filter turkey-latest.osm.pbf amenity=drinking_water --bbox 40.1,37.8,40.3,38.0
osmctl data filter turkey-latest.osm.pbf highway=bus_stop -o geojson > stops.geojson
osmctl data filter turkey-latest.osm.pbf building --type way --bbox 40.20,37.90,40.25,37.93 -o xml > buildings.osm
```

`data extract` is an alias, which reads naturally for a plain area cut:

```sh
osmctl data extract turkey-latest.osm.pbf --bbox 40.20,37.90,40.25,37.93 -o xml > old-city.osm
```

How results are put together:

- Ways and relations come with every node needed to draw them, so `-o xml` opens in JOSM and `-o geojson` opens in QGIS without missing geometry.
- With `--bbox`, ways crossing the edge of the box are kept whole.
- After a tag filter, the extra geometry nodes are stripped of tags so they aren't mistaken for matches. A plain `--bbox` extract keeps all tags.

The file is read twice: once to find matches, then once more to collect the nodes they need. Results are held in memory, so for very large outputs narrow the area or the tags.

## Converting formats

`-o xml`, `-o json` and `-o geojson` convert whatever you extract. To convert a whole small file, extract it with its own bounding box from `data info`.

For heavy processing of planet-scale files (merging, applying diffs, writing `.pbf`), use [osmium-tool](https://osmcode.org/osmium-tool/); osmctl reads its output.
