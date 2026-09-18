# UI for rmfakecloud

## Development

Install dependencies with `pnpm install`, then run `pnpm dev`. The Vite development server runs on port 3001 and proxies `/ui/api` to the backend on port 3000.

- `pnpm build` — TypeScript check and production build.
- `pnpm test:documents` — document sorting, selection, and move validation tests (Node's built-in test runner).
- `pnpm lint` — existing TypeScript lint checks.

## Document browser

The library has grid previews and a sortable list, folder breadcrumbs, recursive folder search, recent documents, and favorites. Folders stay first in either sort direction. Grid and list share the same ordering and selection.

Select items with checkboxes, Shift-click a range, or Ctrl/Command-click a document. The selection toolbar supports move, delete, and downloading original `.rmdoc` files, including documents inside selected folders. Downloads are separate files; your browser may ask you to allow multiple downloads. The operation receipt reports progress and individual failures. Refresh before retrying a failed operation, since a lost connection can obscure a successful server write.

Move through the destination dialog, or drag selected items onto folders, breadcrumbs, or **My library**. Moving a folder inside itself or its descendants is rejected. Deletion includes folder contents and requires confirmation; recovery depends on the sync backend and is not offered by the browser.

Favorites, layout, and sort preferences are saved per account in this browser. They do not sync to other browsers or the tablet. Selection survives sort/layout changes and clears when navigating or searching.

Keyboard shortcuts work while focus is within the document browser and outside a text field:

| Key | Action |
| --- | --- |
| `/` | Focus search |
| Ctrl/Command + A | Select all visible items |
| Shift + click checkbox | Select a range |
| Ctrl/Command + click document | Toggle selection |
| `M` | Move selection |
| Delete | Review deletion |
| Escape | Clear selection or close menu/dialog |
| `?` | Shortcut help |

Right-click opens document actions. The per-item menu provides the same actions for keyboard and touch users. PDF previews load near the viewport and fall back to a document icon if export is unavailable.

The design exploration and implementation boundaries are in [document-browser.md](../docs/design/document-browser.md).

## Dashboard and instance health

The React/Vite UI provides light, dark, and system themes. Shared CSS tokens live
in `src/modern.scss`; API access and responsive page components are separate so
that a future mobile shell can reuse the same authenticated endpoints. This is a
responsive web app, not yet an offline app or native mobile package.

- `/` uses `GET /ui/api/dashboard` for account-scoped document totals and format
  distribution, document metadata sizes, WebSocket connections, and sync events.
- `/health` uses administrator-only `GET /ui/api/health`. It reports the compiled
  version, upstream's documented firmware ceiling (3.27.1), library SQLite ping,
  data-directory logical sizes, WebSocket count, loaded TLS certificate validity,
  and per-account backup policies/latest runs. Release checks are an explicit
  link to upstream; the server does not contact GitHub automatically.
- Sync charts count sync-completed notifications observed by the hub, bucketed
  by UTC day for seven days. They are held in memory and reset on restart. They
  do not measure successful transfers, bytes synchronized, or MQTT clients.
- Storage scanning is read-only and limited to two seconds; incomplete results
  are marked as lower bounds. It does not measure disk capacity or writability.
- TLS terminated at a reverse proxy is not observable from the loaded server
  certificate. Health status explicitly shows this limitation.

Pages refresh on opening and with the Refresh action. Errors retain any previous
snapshot with a stale-data warning; no synthetic chart data is generated.
