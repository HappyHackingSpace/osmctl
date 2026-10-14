## osmctl note list

List notes in an area or matching text

```
osmctl note list [flags]
```

### Examples

```
  osmctl note list --near 37.9144,40.2306 --radius 2000
  osmctl note list --bbox 40.1,37.8,40.3,38.0 --all
  osmctl note list --text "missing road"
```

### Options

```
      --all               include closed notes of any age
      --bbox string       notes in minlon,minlat,maxlon,maxlat
  -h, --help              help for list
  -n, --limit int         maximum number of notes (max 10000) (default 100)
      --near string       notes around lat,lon
  -r, --radius float      radius in meters for --near (default 1000)
      --recently-closed   include notes closed in the last 7 days
      --text string       only notes containing this text
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl note](osmctl_note.md)	 - Report problems and resolve map notes

