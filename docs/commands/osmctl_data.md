## osmctl data

Work with local .osm and .osm.pbf files

### Synopsis

Work offline with OSM files: download regional extracts, inspect them,
and cut out what you need by tag or area.

Supported inputs: .osm.pbf, .osm, .osm.gz and .osm.bz2.

### Options

```
  -h, --help   help for data
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal
* [osmctl data download](osmctl_data_download.md)	 - Download a regional extract as .osm.pbf
* [osmctl data filter](osmctl_data_filter.md)	 - Extract features by tag and/or area from a file
* [osmctl data info](osmctl_data_info.md)	 - Count elements and show the extent and age of a file
* [osmctl data regions](osmctl_data_regions.md)	 - List regional extracts available from Geofabrik

