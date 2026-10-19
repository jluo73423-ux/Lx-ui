# PWA installability verification

This change adds a network-only PWA surface to the login and panel pages. It
does not cache panel data, API responses, credentials, or WebSocket traffic.

## Local checks

Run these commands from the repository root after installing the pinned Node
and Go toolchains:

```text
cd frontend
npm run typecheck
npm run lint
npx vitest run --project unit
npx vitest run --project components
npm run build
cd ..
go test ./...
go build ./...
```

The built binary must serve these paths beneath the configured `webBasePath`:

- `manifest.webmanifest`
- `pwa-register.js`
- `service-worker.js`
- `icons/lx-ui-16.png`
- `icons/lx-ui-24.png`
- `icons/lx-ui-32.png`
- `icons/lx-ui-64.png`
- `icons/lx-ui-192.png`
- `icons/lx-ui-512.png`

The login and panel HTML must contain a manifest link and registration script
whose URLs begin with the same runtime base path. The manifest must contain
`display: "standalone"`, relative `start_url` and `scope`, and all six icon
entries.

## Live rollout checks

Before replacing a server binary, record the current lx-ui binary checksum and
create a timestamped copy of the binary and `/etc/lx-ui/lx-ui.db`. Restart only
the `lx-ui` service after the candidate is staged. Because lx-ui manages Xray as
a child process, the restart can briefly interrupt VPN connections.

After the restart, verify:

1. `lx-ui` is active and its child Xray process is running.
2. The existing panel URL serves HTML with the PWA manifest link.
3. The manifest, registration script, worker, and all six icons return `200`.
4. Login, authenticated API requests, panel navigation, logout, and the panel
   WebSocket all work.
5. At least one VPN client can complete a fresh connection cycle.

If any check fails, restore the exact binary backup, restart lx-ui once, and
repeat the checks against the original build.
