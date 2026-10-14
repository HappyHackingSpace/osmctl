## osmctl auth login

Authorize osmctl with your OpenStreetMap account

### Synopsis

Authorize osmctl with your OpenStreetMap account.

By default this opens your browser and receives the result on
http://127.0.0.1:41117. Use --manual on remote machines to paste the code
instead, or --with-token to read an existing token from standard input.

```
osmctl auth login [flags]
```

### Examples

```
  osmctl auth login
  osmctl --server dev auth login
  osmctl auth login --manual
  osmctl auth login --with-token < token.txt
```

### Options

```
  -h, --help         help for login
      --manual       paste the authorization code instead of using a local redirect
      --with-token   read an access token from standard input
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl auth](osmctl_auth.md)	 - Log in to OpenStreetMap and manage your token

