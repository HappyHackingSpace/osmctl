## osmctl tag

Set or remove tags on an element

### Synopsis

Stage tag changes on an element: key=value sets a tag, key- removes it.

Quote values with spaces. Nothing is uploaded until `osmctl push`.

```
osmctl tag <element> <key=value | key->... [flags]
```

### Examples

```
  osmctl tag node/2187164523 opening_hours="Mo-Sa 08:00-20:00" wheelchair=yes
  osmctl tag w42 name:ku="Kela Amedê" fixme-
```

### Options

```
  -h, --help   help for tag
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal

