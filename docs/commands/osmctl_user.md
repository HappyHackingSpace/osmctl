## osmctl user

Show a user's public profile

### Synopsis

Show a user's public profile by numeric id.

The OSM API looks users up by id only. `osmctl auth status` shows your own.

```
osmctl user <id> [flags]
```

### Examples

```
  osmctl user 1
```

### Options

```
  -h, --help   help for user
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal

