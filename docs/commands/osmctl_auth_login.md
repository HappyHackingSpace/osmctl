## osmctl auth login

Authorize osmctl with your OpenStreetMap account

### Synopsis

Authorize osmctl with your OpenStreetMap account.

This opens your browser; approve osmctl and the login completes by itself.

On a remote machine (over SSH, in a container) open the printed link in any
browser instead. After you approve, that browser shows an error page,
because osmctl is not running there; paste the address it was sent to into
the terminal and the login completes.

--with-token reads an existing token from standard input instead.

```
osmctl auth login [flags]
```

### Examples

```
  osmctl auth login
  osmctl --server dev auth login
  osmctl auth login --no-browser
  osmctl auth login --with-token < token.txt
```

### Options

```
  -h, --help         help for login
      --no-browser   print the authorization link instead of opening a browser
      --with-token   read an access token from standard input
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl auth](osmctl_auth.md)	 - Log in to OpenStreetMap and manage your token

