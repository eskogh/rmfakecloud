import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useHistory, useLocation, useParams } from "react-router-dom";
import { Button, Dropdown, Form, Modal, Spinner } from "react-bootstrap";
import {
  BsArrowDown,
  BsArrowUp,
  BsArrowClockwise,
  BsChevronRight,
  BsClock,
  BsDownload,
  BsFolder,
  BsFolderPlus,
  BsGrid,
  BsListUl,
  BsSearch,
  BsStar,
  BsStarFill,
  BsThreeDots,
  BsTrash,
  BsUpload,
  BsX,
  BsKeyboard,
  BsArrowReturnLeft,
} from "react-icons/bs";
import { toast } from "react-toastify";
import api from "../../services/api.service";
import { useAuthState } from "../../common/useAuthContext";
import File from "./File";
import FileIcon from "./FileIcon";
import Thumbnail from "./Thumbnail";
import Upload from "./Upload";
import {
  ancestors,
  canMove,
  descendants,
  formatDate,
  formatSize,
  indexLibrary,
  readPreference,
  savePreference,
  sortNodes,
  topLevelSelection,
} from "./library";
import styles from "./Browser.module.scss";

const DRAG_TYPE = "application/x-rmfakecloud-documents";
const emptyTree = { Entries: [], Trash: [] };

export default function DocumentBrowser() {
  const {
    state: { user },
  } = useAuthState();
  // Remount account-specific preferences if the signed-in account changes.
  return <Library key={user.UserID} userId={user.UserID} />;
}

function Library({ userId }) {
  const history = useHistory();
  const location = useLocation();
  const { itemId } = useParams();
  const preferenceKey = `rmfakecloud:library:${userId}`;
  const [preferences, setPreferences] = useState(() => {
    const saved = readPreference(preferenceKey, {});
    return {
      view: saved.view === "list" ? "list" : "grid",
      favorites: Array.isArray(saved.favorites) ? saved.favorites : [],
      sort: ["name", "size", "lastModified"].includes(saved.sort)
        ? saved.sort
        : "name",
      direction: saved.direction === "desc" ? "desc" : "asc",
    };
  });
  const [tree, setTree] = useState(emptyTree);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [term, setTerm] = useState("");
  const [selection, setSelection] = useState([]);
  const [dialog, setDialog] = useState(null);
  const [folderName, setFolderName] = useState("");
  const [destination, setDestination] = useState("root");
  const [busy, setBusy] = useState(false);
  const [receipt, setReceipt] = useState(null);
  const [contextMenu, setContextMenu] = useState(null);
  const [dragIds, setDragIds] = useState([]);
  const searchRef = useRef(null);
  const browserRef = useRef(null);
  const anchorRef = useRef(null);
  const busyRef = useRef(false);
  const requestRef = useRef(0);
  const menuRef = useRef(null);
  const menuTrigger = useRef(null);

  const refresh = useCallback(async () => {
    const request = ++requestRef.current;
    setLoading(true);
    try {
      const data = await api.listDocument();
      if (request === requestRef.current) {
        setTree(data);
        setLoadError("");
      }
    } catch (error) {
      if (request === requestRef.current)
        setLoadError(error.message || "Could not load your library.");
    } finally {
      if (request === requestRef.current) setLoading(false);
    }
  }, []);
  useEffect(() => {
    refresh();
    return () => {
      requestRef.current++;
    };
  }, [refresh]);
  useEffect(
    () => savePreference(preferenceKey, preferences),
    [preferenceKey, preferences],
  );
  const index = useMemo(
    () => indexLibrary(tree.Entries || [], tree.Trash || []),
    [tree],
  );
  const node = index.get(itemId || "root");
  const requestedScope = new URLSearchParams(location.search).get("view");
  const scope =
    !itemId && ["recent", "favorites", "trash"].includes(requestedScope)
      ? requestedScope
      : "library";
  const current = scope === "trash" ? index.get("trash") : node;
  const allNodes = useMemo(
    () => [...index.values()].filter((item) => item.parent && !item.inTrash),
    [index],
  );
  const favorites = new Set(preferences.favorites);
  const searching = term.trim().length > 0;
  const visible = useMemo(() => {
    let candidates;
    if (scope === "recent") candidates = allNodes.filter((item) => item.isLeaf);
    else if (scope === "favorites")
      candidates = allNodes.filter((item) =>
        preferences.favorites.includes(item.id),
      );
    else
      candidates = searching
        ? descendants(current?.children || [])
        : current?.children || [];
    candidates = candidates.filter((item) =>
      item.data.name
        .toLocaleLowerCase()
        .includes(term.trim().toLocaleLowerCase()),
    );
    return sortNodes(
      candidates,
      scope === "recent" ? "lastModified" : preferences.sort,
      scope === "recent" ? "desc" : preferences.direction,
      scope !== "recent",
    );
  }, [scope, allNodes, preferences, searching, current, term]);
  const visibleIds = visible.map((item) => item.id);
  const selectedIds = selection.filter((id) => visibleIds.includes(id));
  const title =
    scope === "recent"
      ? "Recently modified"
      : scope === "favorites"
        ? "Favorites"
        : current?.data.name || "Document not found";
  const isFolder = current && !current.isLeaf;
  const canCreate = scope === "library" && isFolder && !current.inTrash;
  const selectedAll =
    visible.length > 0 && selectedIds.length === visible.length;

  useEffect(() => {
    setSelection([]);
    setTerm("");
    setContextMenu(null);
    anchorRef.current = null;
  }, [location.pathname, location.search]);
  useEffect(() => {
    browserRef.current?.focus();
  }, []);
  useEffect(() => {
    setSelection([]);
    anchorRef.current = null;
  }, [term]);

  const setPreference = (key, value) =>
    setPreferences((previous) => ({ ...previous, [key]: value }));
  function open(item) {
    setContextMenu(null);
    history.push(
      item.id === "root"
        ? "/documents"
        : item.id === "trash"
          ? "/documents?view=trash"
          : `/documents/${encodeURIComponent(item.id)}`,
    );
  }
  function toggleFavorite(id) {
    setPreferences((previous) => ({
      ...previous,
      favorites: previous.favorites.includes(id)
        ? previous.favorites.filter((value) => value !== id)
        : [...previous.favorites, id],
    }));
  }
  function select(id, event = {}) {
    if (
      event.shiftKey &&
      anchorRef.current &&
      visibleIds.includes(anchorRef.current)
    ) {
      const start = visibleIds.indexOf(anchorRef.current);
      const end = visibleIds.indexOf(id);
      setSelection((previous) => [
        ...new Set([
          ...previous,
          ...visibleIds.slice(Math.min(start, end), Math.max(start, end) + 1),
        ]),
      ]);
    } else {
      setSelection((previous) =>
        previous.includes(id)
          ? previous.filter((value) => value !== id)
          : [...previous, id],
      );
      anchorRef.current = id;
    }
  }
  function sort(key) {
    setPreferences((previous) => ({
      ...previous,
      sort: key,
      direction:
        previous.sort === key && previous.direction === "asc"
          ? "desc"
          : key === "lastModified" && previous.sort !== key
            ? "desc"
            : "asc",
    }));
  }
  function askMove(ids = selectedIds) {
    setContextMenu(null);
    setDestination("root");
    setDialog({ type: "move", ids: [...ids] });
  }
  function askDelete(ids = selectedIds) {
    setContextMenu(null);
    setDialog({ type: "delete", ids: [...ids] });
  }
  function closeMenu() {
    setContextMenu(null);
    menuTrigger.current?.focus();
  }
  function showMenu(event, item) {
    event.preventDefault();
    menuTrigger.current = event.currentTarget;
    const ids = selectedIds.includes(item.id) ? selectedIds : [item.id];
    setSelection(ids);
    setContextMenu({
      x: Math.max(
        8,
        Math.min(
          event.clientX || event.currentTarget.getBoundingClientRect().left,
          window.innerWidth - 240,
        ),
      ),
      y: Math.max(
        8,
        Math.min(
          event.clientY || event.currentTarget.getBoundingClientRect().bottom,
          window.innerHeight - 285,
        ),
      ),
      item,
      ids,
    });
  }
  useEffect(() => {
    if (!contextMenu) return;
    menuRef.current?.querySelector("button")?.focus();
    const dismiss = (event) => {
      if (!menuRef.current?.contains(event.target)) setContextMenu(null);
    };
    window.addEventListener("pointerdown", dismiss);
    window.addEventListener("resize", closeMenu);
    return () => {
      window.removeEventListener("pointerdown", dismiss);
      window.removeEventListener("resize", closeMenu);
    };
  }, [contextMenu]);

  async function run(action, ids, destinationId) {
    if (busyRef.current || !ids.length) return;
    const nodes = topLevelSelection(ids, index);
    if (!nodes.length) return;
    if (action === "move" && !canMove(ids, destinationId, index)) {
      toast.error("Choose a different folder outside the selected folders.");
      return;
    }
    busyRef.current = true;
    setBusy(true);
    setDialog(null);
    setContextMenu(null);
    const work =
      action === "download"
        ? descendants(nodes).filter((item) => item.isLeaf)
        : nodes;
    const failures = [];
    let completed = 0;
    setReceipt({
      action,
      completed,
      total: work.length,
      failures: [],
      running: true,
    });
    // Run serially: each mutation rewrites the account's sync index.
    for (const item of work) {
      try {
        if (action === "move")
          await api.moveDocument(
            item.id,
            item.data.name,
            destinationId === "root" ? "" : destinationId,
          );
        if (action === "delete") {
          // Delete children first; never orphan them if deleting a child fails.
          for (const child of descendants([item]).reverse())
            await api.deleteDocument(child.id);
        }
        if (action === "download") {
          const blob = await api.download(item.id, "rmdoc");
          const url = URL.createObjectURL(blob);
          const link = document.createElement("a");
          link.href = url;
          link.download = `${item.data.name}.rmdoc`;
          document.body.appendChild(link);
          link.click();
          link.remove();
          setTimeout(() => URL.revokeObjectURL(url), 60000);
        }
        completed++;
      } catch (error) {
        failures.push({
          id: item.id,
          name: item.data.name,
          message: error.message || String(error),
        });
      }
      setReceipt({
        action,
        completed,
        total: work.length,
        failures: [...failures],
        running: true,
      });
    }
    if (action !== "download") await refresh();
    setSelection(failures.map((item) => item.id));
    setReceipt({
      action,
      completed,
      total: work.length,
      failures,
      running: false,
    });
    setBusy(false);
    busyRef.current = false;
    setDragIds([]);
  }

  function dropProps(destinationNode) {
    const eligible = !busy && canMove(dragIds, destinationNode.id, index);
    return {
      "data-drop-target": eligible || undefined,
      onDragOver: (event) => {
        if (eligible && event.dataTransfer.types.includes(DRAG_TYPE)) {
          event.preventDefault();
          event.dataTransfer.dropEffect = "move";
        }
      },
      onDrop: (event) => {
        if (!event.dataTransfer.types.includes(DRAG_TYPE)) return;
        event.preventDefault();
        event.stopPropagation();
        try {
          const ids = JSON.parse(event.dataTransfer.getData(DRAG_TYPE));
          if (Array.isArray(ids) && canMove(ids, destinationNode.id, index))
            run("move", ids, destinationNode.id);
        } catch {
          toast.error("Could not read the dragged selection.");
        }
        setDragIds([]);
      },
    };
  }
  function itemProps(item) {
    return {
      draggable: !busy,
      onDragStart: (event) => {
        const ids = selectedIds.includes(item.id) ? selectedIds : [item.id];
        event.dataTransfer.setData(DRAG_TYPE, JSON.stringify(ids));
        event.dataTransfer.effectAllowed = "move";
        setSelection(ids);
        setDragIds(ids);
      },
      onDragEnd: () => setDragIds([]),
      onContextMenu: (event) => showMenu(event, item),
      ...(!item.isLeaf ? dropProps(item) : {}),
    };
  }

  function keyboard(event) {
    if (
      event.defaultPrevented ||
      event.target.closest(
        'input, textarea, select, [contenteditable="true"]',
      ) ||
      dialog ||
      contextMenu
    )
      return;
    const key = event.key.toLowerCase();
    if ((event.metaKey || event.ctrlKey) && key === "a" && isFolder) {
      event.preventDefault();
      setSelection(visibleIds);
    } else if (key === "/") {
      event.preventDefault();
      searchRef.current?.focus();
    } else if (key === "escape") {
      setSelection([]);
    } else if (key === "?") setDialog({ type: "shortcuts" });
    else if (!event.metaKey && !event.ctrlKey && !event.altKey && !busy) {
      if (key === "delete" && selectedIds.length) {
        event.preventDefault();
        askDelete();
      }
      if (key === "m" && selectedIds.length) {
        event.preventDefault();
        askMove();
      }
    }
  }

  function checkbox(item) {
    return (
      <input
        type="checkbox"
        aria-label={`Select ${item.data.name}`}
        checked={selectedIds.includes(item.id)}
        onChange={() => {}}
        onClick={(event) => select(item.id, event)}
      />
    );
  }
  function star(item) {
    return (
      <button
        className={`${styles.iconButton} ${favorites.has(item.id) ? styles.starred : ""}`}
        aria-label={`${favorites.has(item.id) ? "Unfavorite" : "Favorite"} ${item.data.name}`}
        aria-pressed={favorites.has(item.id)}
        onClick={() => toggleFavorite(item.id)}
      >
        {favorites.has(item.id) ? <BsStarFill /> : <BsStar />}
      </button>
    );
  }
  function more(item) {
    return (
      <Dropdown align="end">
        <Dropdown.Toggle
          className={styles.moreButton}
          variant="link"
          aria-label={`Actions for ${item.data.name}`}
          disabled={busy}
        >
          <BsThreeDots />
        </Dropdown.Toggle>
        <Dropdown.Menu>
          <Dropdown.Item onClick={() => open(item)}>Open</Dropdown.Item>
          <Dropdown.Item onClick={() => toggleFavorite(item.id)}>
            {favorites.has(item.id) ? "Remove favorite" : "Add favorite"}
          </Dropdown.Item>
          <Dropdown.Item
            onClick={() =>
              askMove(selectedIds.includes(item.id) ? selectedIds : [item.id])
            }
          >
            Move…
          </Dropdown.Item>
          <Dropdown.Item
            onClick={() =>
              run(
                "download",
                selectedIds.includes(item.id) ? selectedIds : [item.id],
              )
            }
          >
            Download .rmdoc
          </Dropdown.Item>
          <Dropdown.Divider />
          <Dropdown.Item
            onClick={() =>
              askDelete(selectedIds.includes(item.id) ? selectedIds : [item.id])
            }
          >
            Delete…
          </Dropdown.Item>
        </Dropdown.Menu>
      </Dropdown>
    );
  }

  return (
    <div
      ref={browserRef}
      className={styles.browser}
      onKeyDown={keyboard}
      tabIndex={-1}
    >
      <aside className={styles.sidebar} aria-label="Library navigation">
        <div className={styles.brand}>
          <BsFolder />
          <span>
            Your workspace<small>Make room for ideas.</small>
          </span>
        </div>
        <nav>
          {[
            ["library", "My library", BsFolder, "/documents"],
            ["recent", "Recently modified", BsClock, "/documents?view=recent"],
            ["favorites", "Favorites", BsStar, "/documents?view=favorites"],
            ["trash", "Trash", BsTrash, "/documents?view=trash"],
          ].map(([id, label, Icon, path]) => (
            <button
              key={id}
              className={scope === id ? styles.activeNav : ""}
              aria-current={scope === id ? "page" : undefined}
              onClick={() => history.push(path)}
              {...(id === "library" ? dropProps(index.get("root")) : {})}
            >
              <Icon />
              <span>{label}</span>
              {id === "favorites" && (
                <small>
                  {allNodes.filter((item) => favorites.has(item.id)).length}
                </small>
              )}
            </button>
          ))}
        </nav>
        <div className={styles.sidebarFooter}>
          <span className={styles.avatar}>
            {userId.slice(0, 1).toUpperCase()}
          </span>
          <span title={userId}>
            {userId}
            <small>Personal library</small>
          </span>
        </div>
      </aside>
      <main className={styles.main}>
        <header className={styles.header}>
          <div>
            <span className={styles.eyebrow}>DOCUMENTS</span>
            <h1>{title}</h1>
            <p>
              {scope === "recent"
                ? "Pick up where you left off."
                : scope === "favorites"
                  ? "The things you want to keep close."
                  : scope === "trash"
                    ? "Documents in your device’s trash."
                    : "Your notes, notebooks, and next big ideas."}
            </p>
          </div>
          <div className={styles.headerActions}>
            <button
              className={styles.iconButton}
              title="Keyboard shortcuts (?)"
              aria-label="Keyboard shortcuts"
              onClick={() => setDialog({ type: "shortcuts" })}
            >
              <BsKeyboard />
            </button>
            <button
              className={styles.iconButton}
              title="Refresh library"
              aria-label="Refresh library"
              disabled={loading || busy}
              onClick={refresh}
            >
              <BsArrowClockwise />
            </button>
            {canCreate && (
              <>
                <Button
                  variant="outline-secondary"
                  disabled={busy}
                  onClick={() => {
                    setFolderName("");
                    setDialog({ type: "create" });
                  }}
                >
                  <BsFolderPlus /> New folder
                </Button>
                <Button
                  disabled={busy}
                  onClick={() => setDialog({ type: "upload" })}
                >
                  <BsUpload /> Upload
                </Button>
              </>
            )}
          </div>
        </header>
        {current && scope === "library" && (
          <nav className={styles.breadcrumbs} aria-label="Breadcrumb">
            {ancestors(current).map((item, i, chain) => (
              <span key={item.id}>
                {i > 0 && <BsChevronRight />}
                <button
                  aria-current={i === chain.length - 1 ? "page" : undefined}
                  onClick={() => open(item)}
                  {...(!item.isLeaf ? dropProps(item) : {})}
                >
                  {item.data.name}
                </button>
              </span>
            ))}
          </nav>
        )}
        {loadError && (
          <div role="alert" className={styles.error}>
            {loadError}{" "}
            <button onClick={refresh} disabled={loading}>
              Try again
            </button>
          </div>
        )}
        {loading && tree === emptyTree ? (
          <div className={styles.empty} role="status">
            <Spinner animation="border" />
            <h2>Opening your library…</h2>
          </div>
        ) : !current ? (
          <div className={styles.empty}>
            <BsFolder />
            <h2>Document not found</h2>
            <p>It may have been moved or deleted.</p>
            <Button onClick={() => open(index.get("root"))}>
              Back to library
            </Button>
          </div>
        ) : current.isLeaf && scope === "library" ? (
          <div className={styles.preview}>
            <File key={current.id} file={current} onSelect={open} />
          </div>
        ) : (
          <>
            <div className={styles.toolbar}>
              <label className={styles.search}>
                <BsSearch />
                <input
                  ref={searchRef}
                  type="search"
                  placeholder={
                    scope === "library"
                      ? "Search this folder and subfolders…"
                      : "Search documents…"
                  }
                  aria-label="Search documents"
                  value={term}
                  onChange={(event) => setTerm(event.target.value)}
                />
                <kbd>/</kbd>
              </label>
              <div className={styles.viewControls}>
                <select
                  aria-label="Sort documents"
                  value={
                    scope === "recent"
                      ? "lastModified:desc"
                      : `${preferences.sort}:${preferences.direction}`
                  }
                  disabled={scope === "recent"}
                  onChange={(event) => {
                    const [key, direction] = event.target.value.split(":");
                    setPreferences((previous) => ({
                      ...previous,
                      sort: key,
                      direction,
                    }));
                  }}
                >
                  <option value="name:asc">Name · A–Z</option>
                  <option value="name:desc">Name · Z–A</option>
                  <option value="lastModified:desc">Newest first</option>
                  <option value="lastModified:asc">Oldest first</option>
                  <option value="size:desc">Largest first</option>
                  <option value="size:asc">Smallest first</option>
                </select>
                <div className={styles.viewToggle} aria-label="Document layout">
                  <button
                    aria-label="Grid view"
                    aria-pressed={preferences.view === "grid"}
                    onClick={() => setPreference("view", "grid")}
                  >
                    <BsGrid />
                  </button>
                  <button
                    aria-label="List view"
                    aria-pressed={preferences.view === "list"}
                    onClick={() => setPreference("view", "list")}
                  >
                    <BsListUl />
                  </button>
                </div>
              </div>
            </div>
            <div className={styles.selectionBar}>
              <label>
                <input
                  type="checkbox"
                  aria-label="Select all visible documents"
                  checked={selectedAll}
                  ref={(element) => {
                    if (element)
                      element.indeterminate =
                        selectedIds.length > 0 && !selectedAll;
                  }}
                  onChange={() => setSelection(selectedAll ? [] : visibleIds)}
                  disabled={!visible.length}
                />
                {selectedIds.length
                  ? `${selectedIds.length} selected`
                  : `${visible.length} ${visible.length === 1 ? "item" : "items"}`}
              </label>
              {selectedIds.length > 0 ? (
                <div className={styles.bulkActions}>
                  <button disabled={busy} onClick={() => askMove()}>
                    <BsFolder /> Move
                  </button>
                  <button
                    disabled={busy}
                    onClick={() => run("download", selectedIds)}
                  >
                    <BsDownload /> Download
                  </button>
                  <button disabled={busy} onClick={() => askDelete()}>
                    <BsTrash /> Delete
                  </button>
                  <button
                    aria-label="Clear selection"
                    onClick={() => setSelection([])}
                  >
                    <BsX />
                  </button>
                </div>
              ) : (
                <span>
                  {scope === "recent"
                    ? "Most recently modified first"
                    : "Folders first"}
                  {loading ? " · Refreshing…" : ""}
                </span>
              )}
            </div>
            {visible.length === 0 ? (
              <div className={styles.empty}>
                <BsFolder />
                <h2>
                  {searching
                    ? "No matching documents"
                    : scope === "favorites"
                      ? "Keep your favorites close"
                      : scope === "recent"
                        ? "Your next idea starts here"
                        : "A little room for something new"}
                </h2>
                <p>
                  {searching
                    ? "Try another name or clear your search."
                    : scope === "favorites"
                      ? "Star a document or folder to find it here. Favorites are saved in this browser."
                      : "Your documents will appear here."}
                </p>
                {searching && (
                  <Button
                    variant="outline-secondary"
                    onClick={() => setTerm("")}
                  >
                    Clear search
                  </Button>
                )}
                {canCreate && !searching && (
                  <Button onClick={() => setDialog({ type: "upload" })}>
                    <BsUpload /> Upload your first document
                  </Button>
                )}
              </div>
            ) : preferences.view === "grid" ? (
              <div className={styles.grid}>
                {visible.map((item) => (
                  <article
                    key={item.id}
                    className={`${styles.card} ${selectedIds.includes(item.id) ? styles.selectedCard : ""}`}
                    {...itemProps(item)}
                  >
                    <div className={styles.cardControls}>
                      {checkbox(item)}
                      {star(item)}
                    </div>
                    <button
                      className={styles.coverButton}
                      aria-label={`Open ${item.data.name}`}
                      onClick={(event) =>
                        event.ctrlKey || event.metaKey || event.shiftKey
                          ? select(item.id, event)
                          : open(item)
                      }
                    >
                      <Thumbnail node={item} />
                    </button>
                    <div className={styles.cardBottom}>
                      <div>
                        <button
                          className={styles.documentName}
                          onClick={(event) =>
                            event.ctrlKey || event.metaKey || event.shiftKey
                              ? select(item.id, event)
                              : open(item)
                          }
                          title={item.data.name}
                        >
                          {item.data.name}
                        </button>
                        <small>
                          {item.isLeaf
                            ? formatDate(item.data.lastModified)
                            : `${item.children.length} ${item.children.length === 1 ? "item" : "items"}`}
                          {(scope !== "library" || searching) &&
                            ` · ${item.parent.data.name}`}
                        </small>
                      </div>
                      {more(item)}
                    </div>
                  </article>
                ))}
              </div>
            ) : (
              <div className={styles.tableWrapper}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th aria-label="Selection" />
                      <th
                        aria-sort={
                          scope !== "recent" && preferences.sort === "name"
                            ? preferences.direction === "asc"
                              ? "ascending"
                              : "descending"
                            : "none"
                        }
                      >
                        <button
                          onClick={() => sort("name")}
                          disabled={scope === "recent"}
                        >
                          Name{" "}
                          {preferences.sort === "name" &&
                            (preferences.direction === "asc" ? (
                              <BsArrowUp />
                            ) : (
                              <BsArrowDown />
                            ))}
                        </button>
                      </th>
                      <th
                        className={styles.sizeColumn}
                        aria-sort={
                          preferences.sort === "size" && scope !== "recent"
                            ? preferences.direction === "asc"
                              ? "ascending"
                              : "descending"
                            : "none"
                        }
                      >
                        <button
                          onClick={() => sort("size")}
                          disabled={scope === "recent"}
                        >
                          Size{" "}
                          {preferences.sort === "size" &&
                            (preferences.direction === "asc" ? (
                              <BsArrowUp />
                            ) : (
                              <BsArrowDown />
                            ))}
                        </button>
                      </th>
                      <th
                        className={styles.dateColumn}
                        aria-sort={
                          scope === "recent" ||
                          preferences.sort === "lastModified"
                            ? scope === "recent" ||
                              preferences.direction === "desc"
                              ? "descending"
                              : "ascending"
                            : "none"
                        }
                      >
                        <button
                          onClick={() => sort("lastModified")}
                          disabled={scope === "recent"}
                        >
                          Modified{" "}
                          {(scope === "recent" ||
                            preferences.sort === "lastModified") &&
                            (scope === "recent" ||
                            preferences.direction === "desc" ? (
                              <BsArrowDown />
                            ) : (
                              <BsArrowUp />
                            ))}
                        </button>
                      </th>
                      <th aria-label="Favorites" />
                      <th aria-label="Actions" />
                    </tr>
                  </thead>
                  <tbody>
                    {visible.map((item) => (
                      <tr
                        key={item.id}
                        className={
                          selectedIds.includes(item.id)
                            ? styles.selectedRow
                            : ""
                        }
                        {...itemProps(item)}
                      >
                        <td>{checkbox(item)}</td>
                        <td>
                          <button
                            className={styles.rowName}
                            onClick={(event) =>
                              event.ctrlKey || event.metaKey || event.shiftKey
                                ? select(item.id, event)
                                : open(item)
                            }
                          >
                            <FileIcon file={item.data} />
                            <span>
                              {item.data.name}
                              {(scope !== "library" || searching) && (
                                <small>{item.parent.data.name}</small>
                              )}
                            </span>
                          </button>
                        </td>
                        <td className={styles.sizeColumn}>
                          {item.isLeaf
                            ? formatSize(item.data.size)
                            : `${item.children.length} ${item.children.length === 1 ? "item" : "items"}`}
                        </td>
                        <td className={styles.dateColumn}>
                          {formatDate(item.data.lastModified)}
                        </td>
                        <td>{star(item)}</td>
                        <td>{more(item)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <footer className={styles.hint}>
              A place for everything. Drag documents onto a folder to move them.
              <button onClick={() => setDialog({ type: "shortcuts" })}>
                Keyboard shortcuts
              </button>
            </footer>
          </>
        )}
        {receipt && (
          <div className={styles.receipt} role="status" aria-live="polite">
            <div>
              {receipt.running && <Spinner animation="border" size="sm" />}
              <strong>
                {receipt.action === "download"
                  ? "Download"
                  : receipt.action === "move"
                    ? "Move"
                    : "Delete"}{" "}
                {receipt.running ? "in progress" : "finished"}
              </strong>
              <span>
                {receipt.completed} of {receipt.total}{" "}
                {receipt.action === "download"
                  ? "downloads requested"
                  : "completed"}
              </span>
              {!receipt.running && (
                <button
                  aria-label="Dismiss operation result"
                  onClick={() => setReceipt(null)}
                >
                  <BsX />
                </button>
              )}
            </div>
            {receipt.action === "download" && (
              <small>
                Your browser may ask you to allow multiple downloads.
              </small>
            )}
            {receipt.failures.length > 0 && (
              <details open={!receipt.running}>
                <summary>
                  {receipt.failures.length} failed — refresh before retrying
                </summary>
                <ul>
                  {receipt.failures.map((failure) => (
                    <li key={failure.id}>
                      {failure.name}: {failure.message}
                    </li>
                  ))}
                </ul>
              </details>
            )}
          </div>
        )}
      </main>

      {contextMenu && (
        <div
          ref={menuRef}
          className={styles.contextMenu}
          role="menu"
          aria-label="Document actions"
          style={{ left: contextMenu.x, top: contextMenu.y }}
          onKeyDown={(event) => {
            if (event.key === "Escape" || event.key === "Tab") closeMenu();
            if (["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
              event.preventDefault();
              const items = [
                ...menuRef.current.querySelectorAll("button:not(:disabled)"),
              ];
              const i = items.indexOf(document.activeElement);
              items[
                event.key === "Home"
                  ? 0
                  : event.key === "End"
                    ? items.length - 1
                    : (i +
                        (event.key === "ArrowDown" ? 1 : -1) +
                        items.length) %
                      items.length
              ]?.focus();
            }
          }}
        >
          <button role="menuitem" onClick={() => open(contextMenu.item)}>
            Open
          </button>
          <button
            role="menuitem"
            onClick={() => {
              toggleFavorite(contextMenu.item.id);
              closeMenu();
            }}
          >
            {favorites.has(contextMenu.item.id)
              ? "Remove favorite"
              : "Add favorite"}
          </button>
          <button
            role="menuitem"
            disabled={busy}
            onClick={() => askMove(contextMenu.ids)}
          >
            Move{" "}
            {contextMenu.ids.length > 1
              ? `${contextMenu.ids.length} items`
              : ""}
            …
          </button>
          <button
            role="menuitem"
            disabled={busy}
            onClick={() => run("download", contextMenu.ids)}
          >
            Download .rmdoc
          </button>
          <button
            role="menuitem"
            disabled={busy}
            onClick={() => askDelete(contextMenu.ids)}
          >
            Delete…
          </button>
        </div>
      )}

      <Modal
        show={!!dialog}
        onHide={() => !busy && setDialog(null)}
        centered
        scrollable
      >
        <Modal.Header closeButton={!busy}>
          <Modal.Title>
            {
              {
                move: "Move to folder",
                delete: "Delete selected items?",
                create: "A new place for your ideas",
                upload: "Upload documents",
                shortcuts: "Keyboard shortcuts",
              }[dialog?.type]
            }
          </Modal.Title>
        </Modal.Header>
        <Modal.Body>
          {dialog?.type === "move" && (
            <>
              <p>
                Move {topLevelSelection(dialog.ids, index).length} selected
                items, including folder contents.
              </p>
              <nav
                className={styles.pickerCrumbs}
                aria-label="Destination breadcrumb"
              >
                {ancestors(index.get(destination)).map((item) => (
                  <button key={item.id} onClick={() => setDestination(item.id)}>
                    {item.data.name}
                    <BsChevronRight />
                  </button>
                ))}
              </nav>
              <div className={styles.folderPicker}>
                {sortNodes(
                  index
                    .get(destination)
                    ?.children.filter((item) => !item.isLeaf) || [],
                ).map((item) => {
                  const invalid = topLevelSelection(dialog.ids, index).some(
                    (selected) =>
                      ancestors(item).some(
                        (parent) => parent.id === selected.id,
                      ),
                  );
                  return (
                    <button
                      key={item.id}
                      disabled={invalid}
                      onClick={() => setDestination(item.id)}
                    >
                      <BsFolder />
                      {item.data.name}
                      <BsChevronRight />
                    </button>
                  );
                })}
                {!index
                  .get(destination)
                  ?.children.some((item) => !item.isLeaf) && (
                  <p>No subfolders. You can move your selection here.</p>
                )}
              </div>
            </>
          )}
          {dialog?.type === "delete" && (
            <>
              <p>
                Delete{" "}
                <strong>
                  {descendants(topLevelSelection(dialog.ids, index)).length}{" "}
                  items
                </strong>
                , including all contents of selected folders?
              </p>
              <p>
                This may permanently delete documents, depending on your sync
                backend. This action cannot be undone here.
              </p>
              <ul>
                {topLevelSelection(dialog.ids, index).map((item) => (
                  <li key={item.id}>{item.data.name}</li>
                ))}
              </ul>
            </>
          )}
          {dialog?.type === "create" && (
            <Form
              id="create-folder"
              onSubmit={async (event) => {
                event.preventDefault();
                if (!folderName.trim() || busyRef.current) return;
                busyRef.current = true;
                setBusy(true);
                try {
                  await api.createFolder({
                    name: folderName.trim(),
                    parentId: current.id === "root" ? "" : current.id,
                  });
                  setDialog(null);
                  await refresh();
                } catch (error) {
                  toast.error(error.message || "Could not create folder.");
                } finally {
                  busyRef.current = false;
                  setBusy(false);
                }
              }}
            >
              <Form.Label htmlFor="folder-name">Folder name</Form.Label>
              <Form.Control
                id="folder-name"
                autoFocus
                maxLength={255}
                value={folderName}
                onChange={(event) => setFolderName(event.target.value)}
                placeholder="e.g. A new chapter"
              />
              <Form.Text>Created in {current?.data.name}</Form.Text>
            </Form>
          )}
          {dialog?.type === "upload" && (
            <Upload
              onUploadingChange={(value) => {
                busyRef.current = value;
                setBusy(value);
              }}
              uploadFolder={current.id === "root" ? "" : current.id}
              filesUploaded={() => {
                setDialog(null);
                refresh();
              }}
            />
          )}
          {dialog?.type === "shortcuts" && (
            <dl className={styles.shortcuts}>
              <dt>/</dt>
              <dd>Focus search</dd>
              <dt>Ctrl / ⌘ + A</dt>
              <dd>Select all visible items</dd>
              <dt>Shift + click checkbox</dt>
              <dd>Select a range</dd>
              <dt>Ctrl / ⌘ + click document</dt>
              <dd>Toggle selection</dd>
              <dt>M</dt>
              <dd>Move selected items</dd>
              <dt>Delete</dt>
              <dd>Review deletion</dd>
              <dt>Escape</dt>
              <dd>Clear selection or close a menu</dd>
              <dt>?</dt>
              <dd>Show these shortcuts</dd>
            </dl>
          )}
        </Modal.Body>
        {["move", "delete", "create"].includes(dialog?.type) && (
          <Modal.Footer>
            <Button
              variant="outline-secondary"
              disabled={busy}
              onClick={() => setDialog(null)}
            >
              Cancel
            </Button>
            {dialog?.type === "move" && (
              <Button
                disabled={busy || !canMove(dialog.ids, destination, index)}
                onClick={() => run("move", dialog.ids, destination)}
              >
                <BsArrowReturnLeft /> Move to{" "}
                {index.get(destination)?.data.name}
              </Button>
            )}
            {dialog?.type === "delete" && (
              <Button
                variant="danger"
                onClick={() => run("delete", dialog.ids)}
                disabled={busy}
              >
                Delete items
              </Button>
            )}
            {dialog?.type === "create" && (
              <Button
                type="submit"
                form="create-folder"
                disabled={busy || !folderName.trim()}
              >
                {busy ? "Creating…" : "Create folder"}
              </Button>
            )}
          </Modal.Footer>
        )}
      </Modal>
    </div>
  );
}
