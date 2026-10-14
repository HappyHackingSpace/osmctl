# Searching

osmctl searches with three services. Each is good at something different:

| Command | Service | Good for |
|---|---|---|
| `geocode`, `reverse` | [Nominatim](https://nominatim.org) | places and addresses by name, or the address at a point |
| `find` | [Overpass](https://wiki.openstreetmap.org/wiki/Overpass_API) | every feature with certain tags in an area |
| `query` | Overpass | anything Overpass QL can express |

## Places and addresses

```sh
osmctl geocode "Diyarbakır Surları"
osmctl geocode "Gazi Caddesi, Diyarbakır" --country tr --limit 3
osmctl geocode "Amed" --lang ku
osmctl reverse 37.9144,40.2306
osmctl reverse 37.9144,40.2306 --zoom 10    # 3 = country ... 18 = building
```

Results list the OSM element behind each place, so you can pass it on:

```sh
osmctl get relation/1356759
```

## Finding features by tag

`find` writes the Overpass query for you. Give one or more tag filters, then where to look:

```sh
osmctl find amenity=cafe --near 37.9144,40.2306 --radius 500
osmctl find amenity=drinking_water --in "Diyarbakır"
osmctl find highway=bus_stop --bbox 40.20,37.90,40.25,37.93
```

Every filter must match. Quote filters containing `!`, which bash and zsh otherwise treat as history expansion:

| Filter | Matches |
|---|---|
| `amenity` | the key is present, with any value |
| `'!name'` | the key is absent |
| `amenity=cafe` | exactly this value |
| `'cuisine!=pizza'` | any value but this one, or no such key |
| `name~^Kafe` | value matches a regular expression |
| `'name!~bar'` | value does not match, or no such key |

```sh
# Bus stops nobody has named yet: a mapping to-do list
osmctl find highway=bus_stop '!name' --in "Amed"

# Only ways, as GeoJSON
osmctl find highway=cycleway --in "Berlin" --type way -o geojson > cycleways.geojson
```

`--in` looks the name up with Nominatim and searches inside its boundary. It reports which place it chose; when a name is ambiguous, add more detail (`--in "Sur, Diyarbakır"`) or use `--bbox`.

To learn Overpass, or to adjust a search by hand, print the generated query:

```sh
osmctl find shop=bakery --in "Amed" --show-query
```

## Overpass QL

`query` runs any Overpass QL, given inline, from a file or on standard input:

```sh
osmctl query '[out:json];node["amenity"="cafe"](around:500,37.9144,40.2306);out;'
osmctl query -f stations.overpassql -o geojson > stations.geojson
cat query.overpassql | osmctl query
```

JSON and XML results are parsed, so every output format works. For GeoJSON, ways and relations need their nodes in the result: end the query with

```
out body;
>;
out skel qt;
```

Outputs like `[out:csv(...)]` are printed as Overpass returns them.

[Overpass turbo](https://overpass-turbo.eu) is the best place to develop a query interactively; paste the result into a file and run it with `osmctl query -f`.

## Using other instances

The public Overpass and Nominatim instances are shared and rate limited. osmctl retries busy Overpass responses a couple of times, but for heavy use run your own instance, or use another public one:

```sh
export OSMCTL_OVERPASS_URL=https://overpass.kumi.systems/api/interpreter
export OSMCTL_NOMINATIM_URL=https://nominatim.example.org
```
