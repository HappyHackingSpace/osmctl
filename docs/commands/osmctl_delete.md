## osmctl delete

Delete elements

### Synopsis

Stage elements for deletion. The server refuses to delete a node that a
way still uses, or an element that a relation still lists; delete or edit
those first, in the same push.

```
osmctl delete <element>... [flags]
```

### Examples

```
  osmctl delete node/2187164523
  osmctl delete w42 n100 n101
```

### Options

```
  -h, --help   help for delete
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal

