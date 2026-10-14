## osmctl data download

Download a regional extract as .osm.pbf

### Synopsis

Download the latest extract of a region from Geofabrik, updated daily.
Find region ids with `osmctl data regions`.

```
osmctl data download <region> [flags]
```

### Examples

```
  osmctl data download turkey
  osmctl data download iraq -O data/iraq.osm.pbf
```

### Options

```
  -h, --help                 help for download
  -O, --output-file string   where to save (default <region>-latest.osm.pbf)
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl data](osmctl_data.md)	 - Work with local .osm and .osm.pbf files

