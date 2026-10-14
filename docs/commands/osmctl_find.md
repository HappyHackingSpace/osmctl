## osmctl find

Find map features by tag, near a point, in a box or in a named place

### Synopsis

Find map features by tag without writing Overpass QL.

Every filter must match. Quote filters containing !, which shells
otherwise treat as history expansion:
  key            the tag is present         amenity
  !key           the tag is absent          '!name'
  key=value      exact match                amenity=cafe
  key!=value     anything but               'cuisine!=pizza'
  key~regex      regular expression         name~^Kafe
  key!~regex     does not match             'name!~bar'

--in looks the place up with Nominatim and searches inside its boundary.

```
osmctl find <filter>... (--near lat,lon | --bbox ... | --in place) [flags]
```

### Examples

```
  osmctl find amenity=cafe --near 37.9144,40.2306 --radius 500
  osmctl find amenity=drinking_water --in "Diyarbakır"
  osmctl find highway=bus_stop '!name' --bbox 40.20,37.90,40.25,37.93
  osmctl find tourism=museum --in "Berlin" -o geojson > museums.geojson
  osmctl find shop=bakery --in "Amed" --show-query
```

### Options

```
      --bbox string    search in minlon,minlat,maxlon,maxlat
  -h, --help           help for find
      --in string      search inside a named place (city, district, country)
  -n, --limit int      maximum number of results (default no limit)
      --near string    search around lat,lon
  -r, --radius float   radius in meters for --near (default 1000)
      --show-query     print the generated Overpass QL instead of running it
      --timeout int    Overpass timeout in seconds (default 60)
  -t, --type strings   limit to node, way and/or relation (default all)
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal

