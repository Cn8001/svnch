<script lang="ts">
  import { SvelteSet } from "svelte/reactivity";
  import { relative, statusCode, type StatusEntry } from "./svn";

  let {
    wc,
    entries,
    checked,
    focused,
    onfocus,
    ontoggle,
    onlythese,
    oncontext,
    onlistcontext,
  }: {
    wc: string;
    entries: StatusEntry[];
    checked: Set<string>;
    focused: string;
    onfocus: (path: string) => void;
    ontoggle: (path: string) => void;
    onlythese: (paths: string[]) => void; // Ctrl+click on a box: select just this file, folder or group
    oncontext: (path: string, e: MouseEvent) => void; // a folder row with no change of its own passes its folder path
    onlistcontext: (name: string, paths: string[], e: MouseEvent) => void; // right-click on a changelist's own row
  } = $props();

  type Node = { name: string; key: string; abs: string; entry?: StatusEntry; children: Map<string, Node> };
  type Row =
    | { kind: "group"; key: string; label: string; under: StatusEntry[]; checkable: boolean; changelist: string }
    | { kind: "folder"; key: string; node: Node; depth: number; under: StatusEntry[] }
    | { kind: "file"; key: string; node: Node; depth: number; dir: string };

  const VIEW_KEY = "svnch.changesView";
  let tree = $state(readView());
  // Ignored starts closed, as in Rider: it is rarely what you came for and can be long.
  const collapsed = new SvelteSet<string>(["g:ignored"]);

  function readView(): boolean {
    try {
      return localStorage.getItem(VIEW_KEY) !== "list";
    } catch {
      return true;
    }
  }

  function setView(on: boolean) {
    tree = on;
    try {
      localStorage.setItem(VIEW_KEY, on ? "tree" : "list");
    } catch {}
  }

  function toggleOpen(key: string) {
    if (collapsed.has(key)) collapsed.delete(key);
    else collapsed.add(key);
  }

  let sep = $derived(wc.includes("\\") && !wc.includes("/") ? "\\" : "/");

  // Changelists come first, each its own group, as in Rider; "Changes" is what is in none of them.
  let groups = $derived([
    ...[...new Set(entries.map((e) => e.changelist).filter(Boolean))].sort().map((name) => ({
      key: `cl:${name}`,
      label: name,
      list: entries.filter((e) => e.changelist === name),
      checkable: true,
      changelist: name,
    })),
    { key: "changes", label: "Changes", list: entries.filter((e) => !e.changelist && e.item !== "unversioned" && e.item !== "ignored"), checkable: true, changelist: "" },
    { key: "unversioned", label: "Unversioned Files", list: entries.filter((e) => e.item === "unversioned"), checkable: true, changelist: "" },
    { key: "ignored", label: "Ignored Files", list: entries.filter((e) => e.item === "ignored"), checkable: false, changelist: "" },
  ]);

  function build(prefix: string, list: StatusEntry[]): Node {
    const root: Node = { name: "", key: prefix, abs: wc, children: new Map() };
    for (const e of list) {
      const rel = relative(wc, e.path);
      if (rel === ".") {
        root.children.set(".", { name: ".", key: `${prefix}/.`, abs: wc, entry: e, children: new Map() });
        continue;
      }
      let at = root;
      for (const part of rel.split(/[\\/]/)) {
        let next = at.children.get(part);
        if (!next) {
          next = { name: part, key: `${at.key}/${part}`, abs: at.abs.replace(/[\\/]+$/, "") + sep + part, children: new Map() };
          at.children.set(part, next);
        }
        at = next;
      }
      at.entry = e;
    }
    return root;
  }

  // Folders with nothing to show but one subfolder fold into it, "internal/svn", the way Rider compacts them.
  function compact(n: Node): Node {
    while (!n.entry && n.children.size === 1) {
      const only = [...n.children.values()][0];
      if (!only.children.size || only.entry) break;
      n = { ...only, name: `${n.name}/${only.name}` };
    }
    return n;
  }

  function below(n: Node): StatusEntry[] {
    const out = n.entry ? [n.entry] : [];
    for (const c of n.children.values()) out.push(...below(c));
    return out;
  }

  function sorted(n: Node): Node[] {
    return [...n.children.values()].sort((a, b) => {
      const fa = a.children.size > 0, fb = b.children.size > 0;
      return fa === fb ? a.name.localeCompare(b.name) : fa ? -1 : 1;
    });
  }

  function walk(n: Node, depth: number, out: Row[]) {
    for (const raw of sorted(n)) {
      const c = compact(raw);
      if (c.children.size) {
        out.push({ kind: "folder", key: c.key, node: c, depth, under: below(c) });
        if (!collapsed.has(c.key)) walk(c, depth + 1, out);
      } else {
        out.push({ kind: "file", key: c.key, node: c, depth, dir: "" });
      }
    }
  }

  function flat(prefix: string, list: StatusEntry[], out: Row[]) {
    for (const e of [...list].sort((a, b) => a.path.localeCompare(b.path))) {
      const rel = relative(wc, e.path);
      const cut = Math.max(rel.lastIndexOf("/"), rel.lastIndexOf("\\"));
      const node: Node = { name: cut < 0 ? rel : rel.slice(cut + 1), key: `${prefix}/${e.path}`, abs: e.path, entry: e, children: new Map() };
      out.push({ kind: "file", key: node.key, node, depth: 1, dir: cut < 0 ? "" : rel.slice(0, cut) });
    }
  }

  let rows: Row[] = $derived.by(() => {
    const out: Row[] = [];
    for (const g of groups) {
      if (!g.list.length) continue;
      const key = `g:${g.key}`;
      out.push({ kind: "group", key, label: g.label, under: g.list, checkable: g.checkable, changelist: g.changelist });
      if (collapsed.has(key)) continue;
      if (tree) walk(build(key, g.list), 1, out);
      else flat(key, g.list, out);
    }
    return out;
  });

  function files(n: number): string {
    return `${n} ${n === 1 ? "file" : "files"}`;
  }

  function tone(e: StatusEntry): string {
    if (e.treeConflicted || e.props === "conflicted") return "conflict";
    switch (e.item) {
      case "added":
      case "replaced":
        return "add";
      case "deleted":
      case "missing":
        return "gone";
      case "conflicted":
      case "obstructed":
        return "conflict";
      case "unversioned":
        return "new";
      case "ignored":
        return "ign";
    }
    return "mod";
  }

  function allIn(under: StatusEntry[]): boolean {
    return under.length > 0 && under.every((e) => checked.has(e.path));
  }

  function setAll(under: StatusEntry[]) {
    const on = !allIn(under);
    for (const e of under) if (checked.has(e.path) !== on) ontoggle(e.path);
  }

  // The click knows the modifier, the change that follows does the work, so a plain click keeps the browser's own toggle.
  let exclusive = false;

  function boxClick(ev: MouseEvent) {
    ev.stopPropagation();
    exclusive = ev.ctrlKey || ev.metaKey;
  }

  function boxChange(ev: Event, toggle: () => void, paths: string[]) {
    if (!exclusive) return toggle();
    exclusive = false;
    const box = ev.currentTarget as HTMLInputElement;
    // The box may already have been on; the browser just turned it off, and the same "on" state would not be re-rendered.
    box.checked = true;
    box.indeterminate = false;
    onlythese(paths);
  }

  function open(r: Row) {
    if (r.kind === "group") toggleOpen(r.key);
    else if (r.kind === "folder") {
      toggleOpen(r.key);
      if (r.node.entry) onfocus(r.node.entry.path);
    } else if (r.node.entry) onfocus(r.node.entry.path);
  }

  function key(r: Row, ev: KeyboardEvent) {
    if (ev.key === "Enter") open(r);
    else if (ev.key === " ") {
      ev.preventDefault();
      if (r.kind === "file") r.node.entry && r.node.entry.item !== "ignored" && ontoggle(r.node.entry.path);
      else if (r.kind === "folder" || r.checkable) setAll(r.under);
    } else if (r.kind !== "file" && ev.key === "ArrowLeft") collapsed.add(r.key);
    else if (r.kind !== "file" && ev.key === "ArrowRight") collapsed.delete(r.key);
  }

  function context(r: Row, ev: MouseEvent) {
    ev.preventDefault();
    if (r.kind === "group") {
      if (r.changelist) onlistcontext(r.changelist, r.under.map((e) => e.path), ev);
      return;
    }
    const path = r.node.entry?.path ?? r.node.abs;
    if (r.node.entry) onfocus(path);
    oncontext(path, ev);
  }
</script>

<div class="list">
  <div class="head">
    <span class="title">Changes</span>
    <span class="grow"></span>
    <div class="view" role="group" aria-label="View">
      <button class:on={tree} title="Group by directory" aria-label="Group by directory" onclick={() => setView(true)}>
        <svg viewBox="0 0 16 16" width="14" height="14"><path d="M2 3h5M5 3v8h3M5 7h3M10 7h4M10 11h4" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" /></svg>
      </button>
      <button class:on={!tree} title="Flat list" aria-label="Flat list" onclick={() => setView(false)}>
        <svg viewBox="0 0 16 16" width="14" height="14"><path d="M3 4h10M3 8h10M3 12h10" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" /></svg>
      </button>
    </div>
  </div>
  <div class="rows" role="tree">
    {#each rows as r (r.key)}
      {#if r.kind === "group"}
        <div
          class="row group"
          class:cl={!!r.changelist}
          title={r.changelist ? "Changelist; right-click for its actions" : undefined}
          role="treeitem"
          aria-selected="false"
          aria-expanded={!collapsed.has(r.key)}
          tabindex="-1"
          onclick={() => open(r)}
          onkeydown={(ev) => key(r, ev)}
          oncontextmenu={(ev) => context(r, ev)}
        >
          <svg class="chev" class:closed={collapsed.has(r.key)} viewBox="0 0 16 16" width="12" height="12"><path d="M4 6l4 4 4-4" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" /></svg>
          {#if r.checkable}
            <input
              type="checkbox"
              checked={allIn(r.under)}
              indeterminate={!allIn(r.under) && r.under.some((x) => checked.has(x.path))}
              title="Ctrl+click to select only these"
              onclick={boxClick}
              onchange={(ev) => boxChange(ev, () => setAll(r.under), r.under.map((x) => x.path))}
            />
          {/if}
          {#if r.changelist}<svg class="icon cltag" viewBox="0 0 16 16" width="13" height="13"><path d="M2 2.5h6l6 6-5.5 5.5-6-6z" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round" /><circle cx="5.2" cy="5.7" r="1.1" fill="currentColor" /></svg>{/if}
          <span class="name">{r.label}</span>
          <span class="count">{files(r.under.length)}</span>
        </div>
      {:else}
        {@const e = r.node.entry}
        <div
          class="row {r.kind} {e ? tone(e) : ''}"
          class:focused={!!e && e.path === focused}
          style="--depth: {r.depth}"
          role="treeitem"
          aria-selected={!!e && e.path === focused}
          aria-expanded={r.kind === "folder" ? !collapsed.has(r.key) : undefined}
          tabindex="-1"
          onclick={() => open(r)}
          onkeydown={(ev) => key(r, ev)}
          oncontextmenu={(ev) => context(r, ev)}
        >
          <span class="indent"></span>
          {#if r.kind === "folder"}
            <svg class="chev" class:closed={collapsed.has(r.key)} viewBox="0 0 16 16" width="12" height="12"><path d="M4 6l4 4 4-4" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" /></svg>
          {:else}
            <span class="chev"></span>
          {/if}
          {#if !r.key.startsWith("g:ignored/")}
            <input
              type="checkbox"
              checked={r.kind === "folder" ? allIn(r.under) : !!e && checked.has(e.path)}
              indeterminate={r.kind === "folder" && !allIn(r.under) && r.under.some((x) => checked.has(x.path))}
              title="Ctrl+click to select only this"
              onclick={boxClick}
              onchange={(ev) =>
                boxChange(
                  ev,
                  () => (r.kind === "folder" ? setAll(r.under) : e && ontoggle(e.path)),
                  r.kind === "folder" ? r.under.map((x) => x.path) : e ? [e.path] : [],
                )}
            />
          {/if}
          {#if r.kind === "folder"}
            <svg class="icon dir" viewBox="0 0 16 16" width="15" height="15"><path d="M1.5 3.5h4.5l1.5 1.5h7v8h-13z" fill="currentColor" /></svg>
          {:else}
            <svg class="icon" viewBox="0 0 16 16" width="15" height="15"><path d="M3.5 1.5h6l3 3v10h-9z M9.5 1.5v3h3" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round" /></svg>
          {/if}
          <span class="name" title={r.node.abs}>{r.node.name}</span>
          {#if r.kind === "folder"}
            <span class="count">{files(r.under.length)}</span>
          {:else if r.dir}
            <span class="count dirpath">{r.dir}</span>
          {/if}
          {#if e && (e.props === "modified" || e.treeConflicted)}<span class="tag" title="svn status {statusCode(e)}">{statusCode(e).trim()}</span>{/if}
        </div>
      {/if}
    {:else}
      <p class="empty muted">No local changes.</p>
    {/each}
  </div>
</div>

<style>
  .list { display: flex; flex-direction: column; min-height: 0; height: 100%; }
  .head {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 34px;
    padding: 0 8px 0 12px;
    border-bottom: 1px solid var(--line);
  }
  .title { font-size: 11px; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; color: var(--muted); }
  .grow { flex: 1; }
  .view { display: flex; gap: 2px; }
  .view button { display: grid; place-items: center; width: 26px; height: 24px; padding: 0; border-color: transparent; background: transparent; color: var(--muted); }
  .view button:hover:not(:disabled) { background: var(--hover); border-color: transparent; color: var(--text); }
  .view button.on { color: var(--text); background: var(--hover); }
  .rows { overflow: auto; flex: 1; padding: 4px 0; }
  .row {
    display: flex;
    align-items: center;
    gap: 5px;
    height: 24px;
    padding: 0 10px 0 8px;
    cursor: default;
    white-space: nowrap;
  }
  .row:hover { background: var(--hover); }
  .row.focused { background: var(--accent-soft); box-shadow: inset 2px 0 0 var(--accent); }
  .row:focus-visible { outline: 1px solid var(--accent); outline-offset: -1px; }
  .group { font-weight: 600; margin-top: 2px; }
  .group.cl .cltag { color: var(--accent); }
  .indent { width: calc((var(--depth) - 1) * 16px + 4px); flex: none; }
  .chev { width: 12px; flex: none; color: var(--muted); transition: transform 0.12s; }
  .chev.closed { transform: rotate(-90deg); }
  .icon { flex: none; color: var(--muted); }
  .icon.dir { color: var(--blue); }
  .name { min-width: 0; overflow: hidden; text-overflow: ellipsis; }
  .count { flex: none; margin-left: 4px; font-size: 12px; color: var(--muted); font-weight: normal; }
  .dirpath { flex: 0 1 auto; min-width: 0; overflow: hidden; text-overflow: ellipsis; }
  .tag { margin-left: auto; padding-left: 8px; font-family: var(--mono); font-size: 11px; color: var(--muted); }
  .mod .name { color: var(--mod-ink); }
  .add .name { color: var(--add-ink); }
  .gone .name { color: var(--muted); text-decoration: line-through; }
  .conflict .name { color: var(--danger); font-weight: 600; }
  .new .name { color: var(--new-ink); }
  .ign .name { color: var(--ign-ink); }
  .empty { padding: 12px; margin: 0; }
  input[type="checkbox"] { margin: 0 2px 0 0; flex: none; }
</style>
