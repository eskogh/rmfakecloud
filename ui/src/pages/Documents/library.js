export function indexLibrary(entries = [], trash = []) {
  const index = new Map();
  function visit(data, parent = null, inTrash = false) {
    const node = {
      id: data.id,
      data,
      parent,
      isLeaf: !data.isFolder,
      inTrash,
      children: [],
    };
    index.set(node.id, node);
    node.children = (data.children || []).map((child) =>
      visit(child, node, inTrash),
    );
    return node;
  }
  visit({ id: "root", name: "My library", isFolder: true, children: entries });
  visit(
    { id: "trash", name: "Trash", isFolder: true, children: trash },
    null,
    true,
  );
  return index;
}

const collator = new Intl.Collator(undefined, {
  numeric: true,
  sensitivity: "base",
});
export function sortNodes(
  nodes,
  key = "name",
  direction = "asc",
  foldersFirst = true,
) {
  return [...nodes].sort((a, b) => {
    if (foldersFirst && a.isLeaf !== b.isLeaf) return a.isLeaf ? 1 : -1;
    let order;
    if (key === "lastModified")
      order = timestamp(a.data.lastModified) - timestamp(b.data.lastModified);
    else if (key === "size") order = (a.data.size || 0) - (b.data.size || 0);
    else order = collator.compare(a.data.name, b.data.name);
    return (
      (order ||
        collator.compare(a.data.name, b.data.name) ||
        a.id.localeCompare(b.id)) * (direction === "desc" ? -1 : 1)
    );
  });
}

function timestamp(value) {
  const time = Date.parse(value);
  return Number.isFinite(time) ? time : 0;
}

export function ancestors(node) {
  const result = [];
  for (let parent = node; parent; parent = parent.parent)
    result.unshift(parent);
  return result;
}

export function topLevelSelection(ids, index) {
  const selected = new Set(ids);
  return [...selected]
    .map((id) => index.get(id))
    .filter(
      (node) =>
        node &&
        node.parent &&
        !ancestors(node.parent).some((parent) => selected.has(parent.id)),
    );
}

export function canMove(ids, destinationId, index) {
  const destination = index.get(destinationId);
  if (!ids.length || !destination || destination.isLeaf || destination.inTrash)
    return false;
  const selected = topLevelSelection(ids, index);
  return (
    selected.length > 0 &&
    selected.every(
      (node) =>
        node.parent.id !== destinationId &&
        !ancestors(destination).some((parent) => parent.id === node.id),
    )
  );
}

export function descendants(nodes) {
  return nodes.flatMap((node) => [node, ...descendants(node.children)]);
}

export function formatSize(bytes) {
  if (!Number.isFinite(bytes) || bytes < 0) return "—";
  if (!bytes) return "0 B";
  const unit = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), 3);
  return `${Number((bytes / 1024 ** unit).toFixed(1))} ${["B", "KB", "MB", "GB"][unit]}`;
}

export function formatDate(value) {
  if (!timestamp(value) || timestamp(value) < 0) return "—";
  return new Intl.DateTimeFormat(undefined, { dateStyle: "medium" }).format(
    new Date(value),
  );
}

export function readPreference(key, fallback) {
  try {
    return JSON.parse(localStorage.getItem(key)) ?? fallback;
  } catch {
    return fallback;
  }
}

export function savePreference(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    /* Private browsing can disable storage. */
  }
}
