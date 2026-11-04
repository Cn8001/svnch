<script lang="ts">
  import { Clipboard } from "@wailsio/runtime";
  import type { ListEntry, Server } from "./svn";

  let {
    servers,
    list,
    onsave,
    onremove,
    oncheckout,
    onlog,
    check,
    cancel,
    onlogin,
  }: {
    servers: Server[];
    list: (url: string) => Promise<ListEntry[] | null>; // null when it failed; the caller already reported why
    onsave: (s: Server) => Promise<boolean>;
    onremove: (url: string) => Promise<void>;
    oncheckout: (url: string) => void;
    onlog: (url: string) => void;
    check: (url: string, username: string) => Promise<{ ok: boolean; text: string; username: string }>;
    onlogin: (url: string) => void;
    cancel: () => void; // stops the svn call that is running, for a server that does not answer
  } = $props();

  let server = $state<Server | null>(null);
  let url = $state("");
  let typed = $state("");
  let entries: ListEntry[] = $state([]);
  let selected = $state("");
  let loading = $state(false);
  let form = $state<Server | null>(null);
  let editing = $state("");
  let checked = $state<{ ok: boolean; text: string } | null>(null);
  let checking = $state(false);

  let target = $derived(selected ? child(url, selected) : url);
  let canUp = $derived(!!server && !!url && url.replace(/\/+$/, "") !== server.url.replace(/\/+$/, ""));

  function child(base: string, name: string): string {
    return base.replace(/\/+$/, "") + "/" + encodeURIComponent(name).replace(/%2F/g, "/");
  }

  async function open(to: string) {
    loading = true;
    const r = await list(to);
    loading = false;
    if (r === null) return;
    url = to.replace(/\/+$/, "");
    typed = url;
    selected = "";
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
      <div class="table">
        <div class="row hdr"><span>Name</span><span>Size</span><span>Rev</span><span>Author</span><span>Date</span></div>
        <div class="rows">
          {#each entries as e (e.name)}
            <button
              class="row"
              class:sel={selected === e.name}
              onclick={() => (selected = selected === e.name ? "" : e.name)}
              ondblclick={() => e.kind === "dir" && open(child(url, e.name))}
            >
              <span class="name">{e.name}{e.kind === "dir" ? "/" : ""}</span>
              <span class="muted">{size(e.size)}</span>
              <span>{e.lastCommit.revision ? `r${e.lastCommit.revision}` : ""}</span>
              <span class="muted">{e.lastCommit.author}</span>
              <span class="muted">{e.lastCommit.date ? new Date(e.lastCommit.date).toLocaleDateString() : ""}</span>
            </button>
          {:else}
            <p class="muted empty">{loading ? "Loading..." : url ? "This folder is empty." : "Nothing open."}</p>
          {/each}
        </div>
      </div>
      <div class="tools end">
        <span class="muted where" title={target}>{target}</span>
        <button disabled={!target} onclick={() => Clipboard.SetText(target)}>Copy URL</button>
        <button disabled={!target} onclick={() => onlog(target)}>Log</button>
        <button class="primary" disabled={!target} onclick={() => oncheckout(target)}>Checkout...</button>
      </div>
    {/if}
  </section>
</div>

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
  .form { display: grid; gap: 12px; max-width: 480px; }
  .fine { margin: -6px 0 0; font-size: 12px; }
  .bad { color: var(--danger); }
  .grow { flex: 1; }
</style>
