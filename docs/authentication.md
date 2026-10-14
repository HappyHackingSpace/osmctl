# Authentication

Reading and searching need no account. Logging in is required to:

- push edits
- comment on, close and reopen notes, and create notes under your name
- comment on changesets
- list your own changesets with `--mine`

## Logging in

```sh
osmctl auth login               # openstreetmap.org
osmctl --server dev auth login  # the sandbox needs its own account and login
```

osmctl opens your browser on the OpenStreetMap authorization page. After you approve, the browser returns to osmctl on `http://127.0.0.1:41117`, and osmctl stores the token in your system keychain (macOS Keychain, Windows Credential Manager, or the Secret Service on Linux).

osmctl ships with its own OAuth application for openstreetmap.org, so there is nothing to register. The development server and self-hosted instances need their own; see [below](#oauth-applications-and-self-hosted-instances).

Check who you are logged in as:

```sh
osmctl auth status
```

Log out, which also revokes the token on the server:

```sh
osmctl auth logout
```

osmctl requests these permissions: `read_prefs` (your profile), `write_api` (edit the map), `write_notes` (notes) and `write_changeset_comments` (changeset discussions). OpenStreetMap tokens do not expire; revoke one with `auth logout`, or under **My Settings > OAuth 2 applications** on the website.

## Remote machines

When osmctl runs somewhere your browser can't reach (over SSH, in a container), use the copy and paste flow:

```sh
osmctl auth login --manual
```

1. Open the printed URL in a browser on any device and approve osmctl.
2. The browser is sent to `http://127.0.0.1:41117/callback?code=...`. On that device nothing is listening, so the page fails to load. That is expected.
3. Copy the whole address from the address bar and paste it into the terminal.

osmctl checks that the pasted address belongs to the login it started before using the code.

## CI and machines without a keychain

Set `OSMCTL_TOKEN` to an access token. It takes precedence over the keychain:

```sh
export OSMCTL_TOKEN=...   # from your CI secret store
osmctl auth status
```

To save an existing token in the keychain instead:

```sh
osmctl auth login --with-token < token.txt
```

Treat tokens like passwords: anyone holding one can edit the map as you.

## OAuth applications and self-hosted instances

OAuth applications are registered separately on every OpenStreetMap instance. osmctl includes one for openstreetmap.org. For the [development server](https://master.apis.dev.openstreetmap.org) or a self-hosted instance, register one:

1. Log in to the instance's website and open `/oauth2/applications/new`.
2. Name it `osmctl` and leave **Confidential application** unchecked: osmctl is a public client and uses PKCE.
3. Set the redirect URI to `http://127.0.0.1:41117/callback`. It serves both the browser login and `--manual`.
4. Grant the permissions osmctl requests: `read_prefs`, `write_api`, `write_notes` and `write_changeset_comments`.
5. Save, then pass the **Client ID** to osmctl:

```sh
export OSMCTL_CLIENT_ID=<client id>
osmctl --server dev auth login
```

The client ID of a public application is not a secret.
