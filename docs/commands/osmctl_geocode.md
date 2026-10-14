## osmctl geocode

Find places by name or address (Nominatim)

```
osmctl geocode <place or address> [flags]
```

### Examples

```
  osmctl geocode "Diyarbakır Surları"
  osmctl geocode "Gazi Caddesi, Diyarbakır" --country tr -n 3
  osmctl geocode "Hevsel Gardens" -o geojson
```

### Options

```
  -c, --country strings   limit to ISO 3166-1 country codes, e.g. tr,iq
  -h, --help              help for geocode
  -l, --lang string       preferred result language, e.g. ku, tr, en
  -n, --limit int         maximum number of results (max 40) (default 5)
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal

