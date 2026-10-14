## osmctl member remove

Remove members from a relation

```
osmctl member remove <relation> <element>... [flags]
```

### Examples

```
  osmctl member remove r1234 way/5678
```

### Options

```
  -h, --help   help for remove
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl member](osmctl_member.md)	 - Add or remove members of a relation

