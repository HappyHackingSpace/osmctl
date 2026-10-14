## osmctl changeset download

Download what a changeset changed, as osmChange

### Synopsis

Download the full content of a changeset as osmChange.

The default text output summarizes it; use -o xml for a .osc file that
`osmctl apply` and other tools can read.

```
osmctl changeset download <id> [flags]
```

### Examples

```
  osmctl changeset download 151234567
  osmctl changeset download 151234567 -o xml > cs.osc
```

### Options

```
  -h, --help   help for download
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl changeset](osmctl_changeset.md)	 - Inspect, list, download and discuss changesets

