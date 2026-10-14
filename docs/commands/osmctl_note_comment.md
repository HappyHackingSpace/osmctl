## osmctl note comment

Add a comment to a note

```
osmctl note comment <id> [flags]
```

### Examples

```
  osmctl note comment 4123456 -m "Fixed in changeset 151234567, thanks!"
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

* [osmctl note](osmctl_note.md)	 - Report problems and resolve map notes

