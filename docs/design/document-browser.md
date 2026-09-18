# Modern document browser: design exploration

Improve the existing library around finding, organizing, and acting on documents. Three isolated divergent agents explored logistics, on-call reliability, and a child's physical-world intuition; the session allowed three concurrent worker agents, so the usual five-frame run was reduced to three. No branch received another branch's output. Three subsequent focus passes deepened the strongest candidates.

## Wide set

Scores are novelty, viability, and fit, each out of ten. Ranking uses N × .35 + V × .40 + F × .25. These are design judgments, not measured results.

**Carry a collection to its destination**

- Stage selections in a move queue while browsing destinations. [N7 V9 F9]
- A traveling stack of selected papers. [N8 V9 F9]
- Recent destinations and favorite receiving docks. [N6 V9 F8]
- Touch action tray and a full-screen destination picker. [N5 V9 F9]
- Show the full destination and item count before dropping. [N6 V9 F9]

**Make partial operations legible**

- Batch manifests with progress and individual failures. [N8 V9 F10]
- Persistent operation receipts across navigation. [N7 V8 F9]
- Copyable diagnostic codes with focus restoration. [N6 V6 F6]

**Make every interaction obey the same rules**

- One command pipeline for toolbar, menu, shortcut, and drag/drop. [N7 V10 F10]
- ID-based selections with a count of hidden targets. [N6 V9 F9]
- Keep the last successful view during refresh failures. [N5 V10 F9]

**Navigate by recognition**

- Breadcrumb drop stations that expand on hover. [N7 V6 F8]
- Breadcrumb drawers that reveal sibling destinations. [N8 V6 F8]
- Peek inside folders without entering them. [N7 V7 F7]

**Make exploration reversible or direct**

- One-click return receipts for moves and deletes. [N7 V3 F8] — trap
- Rewind strip for recent changes. [N7 V3 F8] — trap
- Sweep a rectangle to collect documents. [N6 V6 F7]
- Tiny natural-language commands for outcomes. [N8 V4 F6] — trap

## Converge

1. **Shared command pipeline, 8.95.** Centralized validation and execution keep mouse, keyboard, and touch actions consistent.
2. **★ Batch manifests, 8.90.** Per-item failures make ordinary bulk controls substantially more useful when a sync backend partially fails.
3. **Traveling selection, 8.65.** A destination picker lets users carry a fixed selection through folders without hiding targets in the main browser.

Traps:

- Undo and rewind cannot be promised: the newer sync backend can permanently remove documents, and there is no transactional restore endpoint.
- Natural-language commands add ambiguous targeting and considerable implementation cost before the basic library is reliable.
- Rendering every document preview eagerly would export the entire library; previews should follow viewport visibility.

## Focus

### Shared actions

Every action captures stable document IDs. Toolbar actions, context menus, and shortcuts call the same executor. Drag/drop uses the same move validation. Selected descendants collapse into their selected ancestor for moves and deletion. The browser rejects self/descendant destinations, and the server validates the latest visible tree as well. Requests run serially because mutations rewrite the sync index.

**Risk:** validation and mutation are not an atomic transaction across independent clients; storage-level concurrency remains a backend concern.

**First step:** implement and test pure selection normalization and destination validation.

**Child ideas:** derive disabled states from the planner; record per-item outcomes; prevent overlapping mutations; use ID-based selection across layout changes.

### Batch manifests

An operation starts with a frozen target list and a visible count. The receipt updates after each request. Successes and failures are reported separately. Failed targets remain selected where visible, and the library refreshes after mutations. Folder deletion processes descendants before their parent, stopping that branch on error. Downloads expand selected folders into individual original .rmdoc downloads.

**Risk:** a lost response can hide a successful server write; refresh before retrying, and do not claim rollback or exactly-once execution.

**First step:** make API failures reject correctly, then add an operation receipt around the serial executor.

**Child ideas:** grouped failure causes; cancel queued work; reconcile uncertain results; downloadable operation receipts.

### Traveling selection

The move dialog captures selected IDs when opened. Users browse only destination folders, with breadcrumbs to return to ancestors. The primary button names the destination. Self and descendant destinations are disabled. Desktop users can instead drop onto a folder, breadcrumb, or the library navigation item. Main-view navigation and searching clear selection so destructive actions never silently target hidden items.

**Risk:** preserving hidden selections can obscure action scope; this implementation preserves selection inside the picker and across sorting/layout changes, but clears it when the main scope changes.

**First step:** build a folder-only destination picker with cycle checks.

**Child ideas:** recent destination shortcuts; a selection inspector; favorite destinations; hover-to-open folders.

## Provocation

What if filing a note took one gesture from the document preview, without returning to the library?

## Implementation boundaries

- Favorites and layout/sorting preferences are stored per account in browser local storage, not synced to a tablet or other browser.
- Recent is a library-wide document view sorted by modified date; trash is excluded.
- Folder search includes descendants. Sorting uses natural names and keeps folders first in both directions.
- Preview cards lazily request the existing PDF export endpoint. Unsupported exports show a type icon.
- Bulk downloads produce separate .rmdoc files; browser permission for multiple downloads may be required.
- Deletion requires confirmation and does not promise undo.
