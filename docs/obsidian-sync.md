# Obsidian sync (Self-hosted LiveSync)

Obsidian is a desktop/mobile app, so it isn't hosted on the gateway itself.
Instead you run the **CouchDB** app here and sync your vault to it with the
[Self-hosted LiveSync](https://github.com/vrtmrz/obsidian-livesync) community
plugin — real-time, end-to-end-encrypted sync across all your devices.

## 1. Install CouchDB

Install **CouchDB** from the dashboard marketplace. It runs at
`https://couchdb.<your-domain>`.

## 2. Set a strong password

The blueprint ships with `COUCHDB_PASSWORD=changeme`. Change it before syncing
anything real — set it in the app's environment (or via Fauxton at
`https://couchdb.<your-domain>/_utils`) and pick a strong password.

## 3. Enable CORS and LiveSync settings

LiveSync needs CORS enabled and a few tuned settings. Run these against the
CouchDB admin API (replace the host and password), or set the equivalents in
Fauxton → Configuration:

```sh
H="https://admin:PASSWORD@couchdb.your-domain"
curl -X PUT "$H/_node/_local/_config/chttpd/require_valid_user" -d '"true"'
curl -X PUT "$H/_node/_local/_config/chttpd/enable_cors" -d '"true"'
curl -X PUT "$H/_node/_local/_config/cors/credentials" -d '"true"'
curl -X PUT "$H/_node/_local/_config/cors/origins" -d '"app://obsidian.md,capacitor://localhost,http://localhost"'
curl -X PUT "$H/_node/_local/_config/cors/methods" -d '"GET,PUT,POST,HEAD,DELETE"'
curl -X PUT "$H/_node/_local/_config/cors/headers" -d '"accept,authorization,content-type,origin,referer"'
```

Then create the vault database:

```sh
curl -X PUT "$H/obsidian"
```

## 4. Configure the Obsidian plugin

1. In Obsidian, install and enable **Self-hosted LiveSync** (Community plugins).
2. In its settings under Remote Database, set:
   - URI: `https://couchdb.<your-domain>`
   - Username / Password: your CouchDB admin credentials
   - Database name: `obsidian`
3. Set an **end-to-end encryption passphrase**. Your notes are encrypted on-device
   before upload, so the gateway only ever stores ciphertext.
4. Run "Check database configuration" — it should report all green.
5. Enable LiveSync and run the first sync. Repeat this setup on each device using
   the **same passphrase**.

## Notes

- Because the vault is end-to-end encrypted by the plugin, the passphrase is the
  only way to read it — keep it safe; losing it means losing the data.
- CouchDB's data is included in the gateway's backup system (see the blueprint).
- Prefer notes **in the browser**? Install **SilverBullet** instead — an
  Obsidian-like markdown app that runs on the gateway itself, no desktop app or
  sync setup required.
