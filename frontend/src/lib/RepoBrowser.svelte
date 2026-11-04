<script lang="ts">
  import { onMount } from "svelte";
  import { Clipboard, Dialogs } from "@wailsio/runtime";
  import ContextMenu from "./ContextMenu.svelte";
  import { svn, join, type FileContent, type ListEntry, type MenuItem, type Server } from "./svn";

  type Result<T> = { ok: true; value: T } | { ok: false; error: string };

  let {
    servers,
    list,
    onsave,
    onremove,
    oncheckout,
    onlog,
    check,
    start = "",
    cancel,
    onlogin,
    exec,
    known = [],
  }: {
    servers: Server[];
    list: (url: string) => Promise<ListEntry[] | string>; // a string is why it failed
    onsave: (s: Server) => Promise<boolean>;
    onremove: (url: string) => Promise<void>;
    oncheckout: (url: string) => void;
    onlog: (url: string) => void;
    check: (url: string, username: string) => Promise<{ ok: boolean; text: string; username: string }>;
    onlogin: (url: string) => void;
    start?: string; // a url to open straight away, as from the working copy's "Browse on server"
    cancel: () => void; // stops the svn call that is running, for a server that does not answer
    exec: <T>(label: string, make: () => Promise<T>, url: string) => Promise<Result<T>>; // runs through the app's login and status bar
    known?: string[]; // repository urls svnch already knows, such as the open working copy's, offered when a server root cannot be listed
  } = $props();

  let server = $state<Server | null>(null);
  let url = $state("");
  let typed = $state("");
  let entries: ListEntry[] = $state([]);
  let selected = $state("");
  let loading = $state(false);
  let failure = $state("");
  let form = $state<Server | null>(null);
  let editing = $state("");
  let checked = $state<{ ok: boolean; text: string } | null>(null);
  let checking = $state(false);
  let op = $state<{ kind: "delete" | "rename" | "mkdir"; name: string; message: string; error: string; busy: boolean } | null>(null);
  let viewer = $state<{ name: string; url: string; content: FileContent | null; error: string } | null>(null);
  let menu = $state<{ x: number; y: number; items: MenuItem[] } | null>(null);
  let note = $state("");

  // A file larger than this is offered for saving rather than shown; the viewer is for reading, not for assets.
  const VIEW_LIMIT = 2 << 20;

  let target = $derived(selected ? child(url, selected) : url);
  let canUp = $derived(!!server && !!url && url.replace(/\/+$/, "") !== server.url.replace(/\/+$/, ""));

  function child(base: string, name: string): string {
    return base.replace(/\/+$/, "") + "/" + encodeURIComponent(name).replace(/%2F/g, "/");
  }

  async function open(to: string) {
    loading = true;
    failure = "";
    const r = await list(to);
    loading = false;
    if (typeof r === "string") {
      failure = r;
      typed = to;
      return;
    }
    url = to.replace(/\/+$/, "");
    typed = url;
    selected = "";
    viewer = null;
    entries = r.sort((a, b) => (a.kind === b.kind ? a.name.localeCompare(b.name) : a.kind === "dir" ? -1 : 1));
  }

  function pick(s: Server) {
    server = s;
    open(s.url);
  }

  function up() {
    open(url.replace(/\/[^/]*$/, ""));
  }

  function startAdd() {
    form = { name: "", url: typed && !server ? typed : "", username: "" };
    editing = "";
    checked = null;
  }

  function startEdit(s: Server) {
    form = { ...s };
    editing = s.url;
    checked = null;
  }

  onMount(() => {
    if (!start) return;
    server = servers.find((s) => start.startsWith(s.url)) ?? null;
    open(start);
  });

  async function checkConnection() {
    if (!form || !form.url.trim()) return;
    checking = true;
    checked = null;
    const r = await check(form.url, form.username);
    checking = false;
    if (form && r.ok && !form.username && r.username) form.username = r.username;
    checked = r;
  }

  async function save() {
    if (!form || !form.url.trim()) return;
    if (editing && editing !== form.url.trim().replace(/\/+$/, "")) await onremove(editing);
    if (await onsave(form)) {
      const saved = form.url.trim().replace(/\/+$/, "");
      form = null;
      const s = servers.find((x) => x.url === saved);
      if (s) pick(s);
    }
  }

  function opMessage(): string {
    if (!op) return "";
    const name = op.name.trim();
    return op.kind === "delete" ? `Delete ${selected}` : op.kind === "rename" ? `Rename ${selected} to ${name}` : `Add folder ${name}`;
  }

  function startOp(kind: "delete" | "rename" | "mkdir", on = selected) {
    selected = kind === "mkdir" ? selected : on;
    op = { kind, name: kind === "rename" ? on : "", message: "", error: "", busy: false };
    note = "";
  }

  async function runOp() {
    if (!op || op.busy) return;
    const name = op.name.trim();
    const message = op.message.trim() || opMessage();
    const at = url;
    op.busy = true;
    op.error = "";
    const r =
      op.kind === "delete"
        ? await exec(`Deleting ${selected}`, () => svn.DeleteRemote([child(at, selected)], message), at)
        : op.kind === "rename"
          ? await exec(`Renaming ${selected}`, () => svn.MoveRemote(child(at, selected), child(at, name), message), at)
          : await exec(`Creating ${name}`, () => svn.MakeDir(child(at, name), message), at);
    if (!r.ok) {
      if (op) {
        op.busy = false;
        op.error = r.error;
      }
      return;
    }
    op = null;
    await open(at);
    note = `Done, committed as r${r.value}.`;
  }

  async function view(e: ListEntry) {
    const u = child(url, e.name);
    viewer = { name: e.name, url: u, content: null, error: "" };
    if (e.size > VIEW_LIMIT) {
      viewer.content = { text: "", binary: true, size: e.size };
      return;
    }
    const r = await exec(`Opening ${e.name}`, () => svn.Cat(u), u);
    if (viewer?.url !== u) return;
    if (r.ok) viewer.content = r.value;
    else viewer.error = r.error;
  }

  async function saveAs(u: string, name: string, dir: boolean) {
    let dest = "";
    if (dir) {
      const parent = await Dialogs.OpenFile({ Title: `Export ${name} into`, CanChooseDirectories: true, CanChooseFiles: false, CanCreateDirectories: true, ButtonText: "Export here" });
      if (typeof parent === "string" && parent) dest = join(parent, name);
    } else {
      dest = await Dialogs.SaveFile({ Title: "Save from the repository", Filename: name, ButtonText: "Save", CanCreateDirectories: true });
    }
    if (!dest) return;
    const r = await exec(`Saving ${name}`, () => svn.Export(u, dest), u);
    note = r.ok ? `Saved to ${dest}` : r.error;
  }

  function entryItems(e: ListEntry | undefined): MenuItem[] {
    if (!e) {
      return [
        { label: "New folder...", action: () => startOp("mkdir"), disabled: !url },
        { label: "Copy URL", action: () => Clipboard.SetText(url), disabled: !url },
        { label: "Refresh", action: () => url && open(url), disabled: !url },
      ];
    }
    const u = child(url, e.name);
    const dir = e.kind === "dir";
    return [
      dir ? { label: "Open folder", action: () => open(u) } : { label: "Open", action: () => view(e) },
      { label: dir ? "Export..." : "Save as...", action: () => saveAs(u, e.name, dir) },
      ...(dir ? [{ label: "Checkout...", action: () => oncheckout(u) }] : []),
      { separator: true },
      { label: "Copy URL", action: () => Clipboard.SetText(u) },
      { label: "Show log", action: () => onlog(u) },
      { separator: true },
      { label: "Rename / move...", action: () => startOp("rename", e.name) },
      { label: "Delete...", action: () => startOp("delete", e.name) },
      { label: "New folder...", action: () => startOp("mkdir") },
    ];
  }

  function size(n: number): string {
    if (!n) return "";
    const units = ["B", "KB", "MB", "GB"];
    let i = 0;
    while (n >= 1024 && i < units.length - 1) {
      n /= 1024;
      i++;
    }
    return `${n < 10 && i ? n.toFixed(1) : Math.round(n)} ${units[i]}`;
  }
</script>

<div class="browser">
  <aside>
    <div class="head">Servers</div>
    <ul>
      {#each servers as s (s.url)}
        <li>
          <button class="srv" class:sel={server?.url === s.url} title={s.url} onclick={() => pick(s)} ondblclick={() => startEdit(s)}>
            <b>{s.name}</b>
            <span class="muted">{s.url}</span>
          </button>
        </li>
      {:else}
        <li class="muted empty">No servers yet. Add your server's address or a repository URL.</li>
      {/each}
    </ul>
    <div class="tools">
      <button onclick={startAdd}>Add...</button>
      <button disabled={!server} onclick={() => server && startEdit(server)}>Edit...</button>
      <button disabled={!server} onclick={() => server && onlogin(server.url)}>Log in...</button>
      <button disabled={!server} onclick={async () => { if (server) { await onremove(server.url); server = null; entries = []; url = ""; } }}>Remove</button>
    </div>
  </aside>

  <section>
    {#if form}
      <form class="form" onsubmit={(e) => { e.preventDefault(); save(); }}>
        <b>{editing ? "Edit server" : "Add server"}</b>
        <label><span>Server or repository URL</span><input bind:value={form.url} oninput={() => (checked = null)} placeholder="https://svn.example.org/svn" /></label>
        <p class="muted fine">Give the server's address to pick from the repositories on it, or one repository's own URL.</p>
        <label><span>Name (optional)</span><input bind:value={form.name} placeholder="Work" /></label>
        <label><span>Username (optional)</span><input bind:value={form.username} /></label>
        {#if checked}<p class="fine" class:bad={!checked.ok}>{checked.text}</p>{/if}
        <div class="tools">
          {#if checking}
            <button type="button" onclick={cancel}>Stop checking</button>
          {:else}
            <button type="button" disabled={!form.url.trim()} onclick={checkConnection}>Check connection</button>
          {/if}
          <span class="grow"></span>
          <button type="button" onclick={() => (form = null)}>Cancel</button>
          <button class="primary" type="submit" disabled={!form.url.trim()}>Save</button>
        </div>
      </form>
    {:else}
      <form class="path" onsubmit={(e) => { e.preventDefault(); if (typed.trim()) open(typed.trim()); }}>
        <button type="button" disabled={!canUp || loading} onclick={up}>Up</button>
        <input bind:value={typed} placeholder="Pick a server, or type a URL" />
        {#if loading}
          <button type="button" onclick={cancel}>Stop</button>
        {:else}
          <button type="submit" disabled={!typed.trim()}>Go</button>
        {/if}
      </form>
      {#if op}
        <form class="form op" onsubmit={(e) => { e.preventDefault(); runOp(); }}>
          <b>{op.kind === "delete" ? `Delete ${selected}` : op.kind === "rename" ? `Rename or move ${selected}` : "New folder"}</b>
          {#if op.kind === "delete"}
            <p>Removes <code>{selected}</code> from the server for everyone, in one commit. It stays in the history, so the log can bring it back.</p>
          {:else}
            <!-- svelte-ignore a11y_autofocus -->
            <label><span>{op.kind === "rename" ? "New name (a/b moves it into folder a)" : "Folder name"}</span><input bind:value={op.name} autofocus /></label>
            {#if op.name.trim()}<p class="muted fine mono">{child(url, op.name.trim())}</p>{/if}
          {/if}
          <label><span>Commit message</span><input bind:value={op.message} placeholder={opMessage()} /></label>
          {#if op.error}<p class="bad">{op.error}</p>{/if}
          <div class="tools">
            <span class="grow"></span>
            <button type="button" onclick={() => (op = null)}>Cancel</button>
            <button class="primary" type="submit" disabled={op.busy || (op.kind !== "delete" && (!op.name.trim() || op.name.trim() === selected))}>
              {op.busy ? "Working..." : op.kind === "delete" ? "Delete" : op.kind === "rename" ? "Rename" : "Create"}
            </button>
          </div>
        </form>
      {:else if viewer}
        <div class="viewer">
          <div class="tools vhead">
            <button onclick={() => (viewer = null)}>Back</button>
            <b class="where" title={viewer.url}>{viewer.name}</b>
            {#if viewer.content && !viewer.content.binary}<button onclick={() => viewer?.content && Clipboard.SetText(viewer.content.text)}>Copy</button>{/if}
            <button onclick={() => viewer && saveAs(viewer.url, viewer.name, false)}>Save as...</button>
          </div>
          {#if viewer.error}
            <p class="bad empty">{viewer.error}</p>
          {:else if !viewer.content}
            <p class="muted empty">Opening...</p>
          {:else if viewer.content.binary}
            <p class="muted empty">
              {viewer.content.size > VIEW_LIMIT ? "This file is too large to show here" : "This is not a text file"} ({size(viewer.content.size)}).
              Save it to open it in its own program.
            </p>
          {:else}
            <pre class="content">{viewer.content.text}</pre>
          {/if}
        </div>
      {:else}
      <div class="table" role="presentation" oncontextmenu={(ev) => { ev.preventDefault(); menu = { x: ev.clientX, y: ev.clientY, items: entryItems(undefined) }; }}>
        <div class="row hdr"><span>Name</span><span>Size</span><span>Rev</span><span>Author</span><span>Date</span></div>
        <div class="rows">
          {#each entries as e (e.name)}
            <button
              class="row"
              class:sel={selected === e.name}
              onclick={() => (selected = selected === e.name ? "" : e.name)}
              ondblclick={() => (e.kind === "dir" ? open(child(url, e.name)) : view(e))}
              oncontextmenu={(ev) => { ev.preventDefault(); ev.stopPropagation(); selected = e.name; menu = { x: ev.clientX, y: ev.clientY, items: entryItems(e) }; }}
            >
              <span class="name">{e.name}{e.kind === "dir" ? "/" : ""}</span>
              <span class="muted">{size(e.size)}</span>
              <span>{e.lastCommit.revision ? `r${e.lastCommit.revision}` : ""}</span>
              <span class="muted">{e.lastCommit.author}</span>
              <span class="muted">{e.lastCommit.date ? new Date(e.lastCommit.date).toLocaleDateString() : ""}</span>
            </button>
          {:else}
            {#if failure && !loading}
              <div class="empty failed">
                <b>Could not open {typed}</b>
                <p>{failure}</p>
                {#if typed.startsWith("http://")}<p class="muted">Many servers, VisualSVN Server among them, only answer on https://. Try {typed.replace("http://", "https://")}</p>{/if}
                {#each known.filter((k) => k.startsWith(typed.replace(/\/+$/, "") + "/")) as k}
                  <p class="muted">Your open working copy is in this repository on that server:</p>
                  <div><button onclick={() => open(k)}>Open {k.slice(typed.replace(/\/+$/, "").length + 1)}</button></div>
                {/each}
              </div>
            {:else}
              <p class="muted empty">{loading ? "Loading..." : url ? "This folder is empty." : "Pick a server on the left, or type an address above."}</p>
            {/if}
          {/each}
        </div>
      </div>
      {/if}
      {#if note}<p class="fine note">{note}</p>{/if}
      <div class="tools end">
        <span class="muted where" title={target}>{target}</span>
        <button disabled={!target} onclick={() => Clipboard.SetText(target)}>Copy URL</button>
        <button disabled={!target} onclick={() => onlog(target)}>Log</button>
        <button
          disabled={!url || !!op}
          onclick={(ev) => {
            const r = (ev.currentTarget as HTMLElement).getBoundingClientRect();
            menu = { x: r.left, y: r.bottom + 4, items: entryItems(entries.find((x) => x.name === selected)) };
          }}
        >More...</button>
        <button class="primary" disabled={!target} onclick={() => oncheckout(target)}>Checkout...</button>
      </div>
    {/if}
  </section>
</div>

{#if menu}<ContextMenu x={menu.x} y={menu.y} items={menu.items} onclose={() => (menu = null)} />{/if}

<style>
  .browser { display: grid; grid-template-columns: 240px 1fr; gap: 12px; height: 60vh; }
  aside, section { display: flex; flex-direction: column; gap: 8px; min-height: 0; min-width: 0; }
  aside ul { flex: 1; margin: 0; padding: 4px; list-style: none; overflow: auto; border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); }
  .head { font-size: 11px; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; color: var(--muted); }
  .srv, .row {
    display: block;
    width: 100%;
    border: 0;
    border-radius: 4px;
    background: transparent;
    text-align: left;
    padding: 5px 8px;
  }
  .srv:hover:not(:disabled), .row:hover:not(:disabled) { background: var(--hover); }
  .srv span { display: block; overflow: hidden; text-overflow: ellipsis; font-size: 12px; }
  .sel, .sel:hover:not(:disabled) { background: var(--accent-soft) !important; box-shadow: inset 2px 0 0 var(--accent); }
  .tools { display: flex; gap: 6px; flex-wrap: wrap; }
  .tools.end { align-items: center; }
  .where { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; font-family: var(--mono); font-size: 12px; }
  .path { display: flex; gap: 6px; }
  .path input { flex: 1; min-width: 0; font-family: var(--mono); font-size: 12px; }
  .table { flex: 1; display: flex; flex-direction: column; min-height: 0; border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); overflow: hidden; }
  .rows { overflow: auto; flex: 1; padding: 4px; }
  .row { display: grid; grid-template-columns: 1fr 6em 5em 8em 7em; gap: 8px; }
  .hdr { font-size: 11px; font-weight: 700; letter-spacing: 0.05em; text-transform: uppercase; color: var(--muted); background: var(--surface); border-bottom: 1px solid var(--line); border-radius: 0; padding: 6px 12px; }
  .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .empty { padding: 10px; margin: 0; }
  .failed { display: grid; gap: 6px; }
  .failed b { color: var(--danger); overflow-wrap: anywhere; }
  .failed p { margin: 0; overflow-wrap: anywhere; user-select: text; }
  .form { display: grid; gap: 12px; max-width: 480px; }
  .fine { margin: -6px 0 0; font-size: 12px; }
  .bad { color: var(--danger); }
  .grow { flex: 1; }
  .op { max-width: none; flex: 1; align-content: start; }
  .op p { margin: 0; }
  .mono { font-family: var(--mono); font-size: 12px; }
  .note { margin: 0; color: var(--muted); overflow-wrap: anywhere; user-select: text; }
  .viewer { flex: 1; display: flex; flex-direction: column; gap: 8px; min-height: 0; }
  .vhead { align-items: center; flex-wrap: nowrap; }
  .content { flex: 1; overflow: auto; margin: 0; padding: 10px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); line-height: 1.5; }
</style>
