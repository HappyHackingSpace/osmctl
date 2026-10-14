## osmctl changeset comment

Comment on a changeset's discussion

```
osmctl changeset comment <id> -m <text> [flags]
```

### Examples

```
  osmctl changeset comment 151234567 -m "Thanks! Could you add a source tag?"
```

### Options

```
  -h, --help             help for comment
  -m, --message string   comment text
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl changeset](osmctl_changeset.md)	 - Inspect, list, download and discuss changesets

