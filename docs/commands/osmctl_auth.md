## osmctl auth

Log in to OpenStreetMap and manage your token

### Synopsis

Log in to OpenStreetMap with OAuth 2.0.

The token is stored in your system keychain, separately for each server.
On machines without a keychain (CI, containers) set OSMCTL_TOKEN instead.

### Options

```
  -h, --help   help for auth
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json, xml, geojson (default "text")
  -s, --server string   OSM instance: dev, osm, or a URL (env OSMCTL_SERVER) (default "osm")
```

### SEE ALSO

* [osmctl](osmctl.md)	 - Read, query, edit and contribute to OpenStreetMap from the terminal
* [osmctl auth login](osmctl_auth_login.md)	 - Authorize osmctl with your OpenStreetMap account
* [osmctl auth logout](osmctl_auth_logout.md)	 - Revoke the token and remove it from the keychain
* [osmctl auth status](osmctl_auth_status.md)	 - Show who you are logged in as

