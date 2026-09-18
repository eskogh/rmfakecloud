import test from "node:test";
import assert from "node:assert/strict";
import {
  indexLibrary,
  sortNodes,
  topLevelSelection,
  canMove,
  descendants,
  formatSize,
  formatDate,
} from "./library.js";

const tree = indexLibrary(
  [
    {
      id: "a",
      name: "Folder 10",
      isFolder: true,
      children: [
        {
          id: "b",
          name: "Nested",
          isFolder: true,
          children: [{ id: "c", name: "Notes 2", size: 20 }],
        },
      ],
    },
    { id: "d", name: "Folder 2", isFolder: true, children: [] },
    {
      id: "e",
      name: "Notes 10",
      size: 10,
      lastModified: "2026-01-02T00:00:00Z",
    },
    {
      id: "f",
      name: "Notes 2",
      size: 200,
      lastModified: "2026-01-01T00:00:00Z",
    },
  ],
  [{ id: "deleted", name: "Deleted" }],
);
const ids = (nodes) => nodes.map((node) => node.id);

test("natural sorting keeps folders first in both directions without mutating source", () => {
  const nodes = tree.get("root").children;
  assert.deepEqual(ids(sortNodes(nodes)), ["d", "a", "f", "e"]);
  assert.deepEqual(ids(sortNodes(nodes, "name", "desc")), ["a", "d", "e", "f"]);
  assert.deepEqual(ids(nodes), ["a", "d", "e", "f"]);
});
test("sorts real sizes and dates; recent order can omit folder grouping", () => {
  assert.deepEqual(ids(sortNodes(tree.get("root").children, "size", "desc")), [
    "a",
    "d",
    "f",
    "e",
  ]);
  assert.deepEqual(
    ids(sortNodes(tree.get("root").children, "lastModified", "desc", false)),
    ["e", "f", "a", "d"],
  );
});
test("selection eliminates descendants, duplicate IDs, stale IDs, and synthetic roots", () => {
  assert.deepEqual(
    ids(topLevelSelection(["c", "a", "b", "a", "unknown"], tree)),
    ["a"],
  );
  assert.deepEqual(ids(topLevelSelection(["root", "trash"], tree)), []);
});
test("moves reject self, descendants, files, missing destinations and unchanged parents", () => {
  for (const destination of ["a", "b", "c", "e", "missing", "trash", "root"]) {
    assert.equal(canMove(["a"], destination, tree), false, destination);
  }
  assert.equal(canMove(["a", "b"], "d", tree), true);
  assert.equal(canMove(["c"], "root", tree), true);
  assert.equal(canMove(["deleted"], "root", tree), true);
  assert.equal(canMove([], "d", tree), false);
});
test("recursive actions include nested contents once after selection normalization", () => {
  assert.deepEqual(ids(descendants(topLevelSelection(["a", "b", "c"], tree))), [
    "a",
    "b",
    "c",
  ]);
  assert.equal(tree.get("deleted").inTrash, true);
});
test("missing metadata does not become NaN, invalid dates, or negative sizes", () => {
  assert.equal(formatSize(undefined), "—");
  assert.equal(formatSize(-1), "—");
  assert.equal(formatSize(0), "0 B");
  assert.equal(formatSize(1024), "1 KB");
  assert.equal(formatDate("invalid"), "—");
  assert.equal(formatDate("0001-01-01T00:00:00Z"), "—");
});
