<script lang="ts">
  import { onMount } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import { Application, Browser, Clipboard, Dialogs } from "@wailsio/runtime";
  import MenuBar from "./lib/MenuBar.svelte";
  import FileList from "./lib/FileList.svelte";
  import DiffView from "./lib/DiffView.svelte";
  import ContextMenu from "./lib/ContextMenu.svelte";
  import Modal from "./lib/Modal.svelte";
  import {
    svn,
    AppService,
    UpdateService,
    type AppInfo,
    CheckoutRequest,
    CommitRequest,
    Depth,
    DiffRequest,
    LogRequest,
    MergeRequest,
    errorText,
    hasChanges,
    isVersioned,
    join,
    relative,
    type Info,
    type LogEntry,
    type MenuItem,
    type StatusEntry,
  } from "./lib/svn";

  const REPO_URL = "https://github.com/Cn8001/svnch";
  const LAST_WC = "svnch.lastWorkingCopy";

  let wc = $state("");
  let info: Info | null = $state(null);
  let entries: StatusEntry[] = $state([]);
  const checked = new SvelteSet<string>();
  let focused = $state("");
  let diffTitle = $state("");
  let diffText = $state("");
  let diffLoading = $state(false);
  let diffSeq = 0;
  let message = $state("");

  let busy = $state("");
  let cancelBusy: (() => void) | null = $state(null);
  let notice = $state("");
  let error = $state("");
  let errorFull = $state("");

  let ctxMenu: { x: number; y: number; items: MenuItem[] } | null = $state(null);

  type ModalKind = "checkout" | "move" | "confirm" | "merge" | "shelve" | "unshelve" | "log" | "help" | "text" | "about" | "settings";

  let appInfo: AppInfo | null = $state(null);
  let updateNote = $state("");
  let modal: ModalKind | null = $state(null);

  let co = $state({ url: "", path: "", revision: "", depth: Depth.DepthDefault });
  let mv = $state({ from: "", to: "" });
  let confirm = $state({ title: "", text: "", ok: "", run: () => {} });
  let mg = $state({ source: "", change: "", dryRun: true, output: "" });
  let sh = $state({ name: "", message: "" });
  let shelves: string[] = $state([]);
  let log = $state({ target: "", entries: [] as LogEntry[], selected: 0 });
  let help = $state({ topic: "", text: "" });
  let textBox = $state({ title: "", text: "" });

  const failed = Symbol("failed");

  // Every svn call goes through here so the status bar, the cancel button and error reporting behave the same.
  async function run<T>(label: string, p: Promise<T> & { cancel?: () => void }): Promise<T | typeof failed> {
    let cancelled = false;
    busy = label;
    error = "";
    notice = "";
    cancelBusy = () => {
      cancelled = true;
      p.cancel?.();
    };
    try {
      return await p;
    } catch (e) {
      error = cancelled ? "Cancelled." : errorText(e);
      errorFull = cancelled ? "" : e instanceof Error ? e.message : String(e);
      return failed;
    } finally {
      busy = "";
      cancelBusy = null;
    }
  }

  function entry(path: string): StatusEntry | undefined {
    return entries.find((e) => e.path === path);
  }

  function remember(path: string) {
    try {
      localStorage.setItem(LAST_WC, path);
    } catch {}
  }

  async function openFolder() {
    const dir = await Dialogs.OpenFile({
      Title: "Open working copy",
      ButtonText: "Open",
      CanChooseDirectories: true,
      CanChooseFiles: false,
    });
    if (dir) await openWC(dir);
  }

  async function openWC(dir: string) {
    const i = await run(`Reading ${dir}`, svn.Info(dir));
    if (i === failed) return;
    info = i;
    wc = i.workingCopyRoot || dir;
    remember(wc);
    checked.clear();
    clearDiff();
    await refresh();
  }

  async function reloadInfo() {
    try {
      info = await svn.Info(wc);
    } catch {}
  }

  async function refresh() {
    if (!wc) return;
    const r = await run("Reading status", svn.Status(wc));
    if (r === failed) return;
    entries = r.sort((a, b) => a.path.localeCompare(b.path));
    for (const p of [...checked]) if (!entry(p)) checked.delete(p);
    if (focused && !entry(focused)) clearDiff();
    else if (focused) await showDiff(focused);
  }

  function clearDiff() {
    focused = "";
    diffTitle = "";
    diffText = "";
  }

  async function showDiff(path: string) {
    focused = path;
    const e = entry(path);
    diffTitle = relative(wc, path);
    diffText = "";
    if (e && !isVersioned(e)) {
      diffTitle += "  (unversioned, add it to track it)";
      return;
    }
    if (e?.item === "missing") {
      diffTitle += "  (missing on disk)";
      return;
    }
    const seq = ++diffSeq;
    diffLoading = true;
    try {
      const text = await svn.Diff(new DiffRequest({ target: path, change: 0 }));
      if (seq === diffSeq) diffText = text;
    } catch (err) {
      if (seq === diffSeq) error = errorText(err);
    } finally {
      if (seq === diffSeq) diffLoading = false;
    }
  }

  async function showRevisionDiff(target: string, rev: number) {
    const seq = ++diffSeq;
    focused = "";
    diffTitle = `r${rev}  ${relative(wc, target)}`;
    diffText = "";
    diffLoading = true;
    try {
      const text = await svn.Diff(new DiffRequest({ target, change: rev }));
      if (seq === diffSeq) diffText = text;
    } catch (err) {
      if (seq === diffSeq) error = errorText(err);
    } finally {
      if (seq === diffSeq) diffLoading = false;
    }
  }

  // An action on a checked row applies to every checked row, the way file managers treat a selection.
  function targetsFor(path: string): string[] {
    return checked.has(path) && checked.size > 1 ? [...checked] : [path];
  }

  function ask(title: string, text: string, ok: string, then: () => void) {
    confirm = { title, text, ok, run: then };
    modal = "confirm";
  }

  function names(paths: string[]): string {
    const shown = paths.slice(0, 8).map((p) => "  " + relative(wc, p));
    if (paths.length > 8) shown.push(`  ...and ${paths.length - 8} more`);
    return shown.join("\n");
  }

  async function add(paths: string[]) {
    if ((await run("Adding", svn.Add(paths))) !== failed) notice = `Added ${paths.length} item(s).`;
    await refresh();
  }

  function revert(paths: string[]) {
    ask("Revert", `Throw away local changes in:\n\n${names(paths)}`, "Revert", async () => {
      if ((await run("Reverting", svn.Revert(paths))) !== failed) notice = `Reverted ${paths.length} item(s).`;
      await refresh();
    });
  }

  function remove(paths: string[], keepLocal: boolean) {
    const text = keepLocal
      ? `Stop versioning these, but keep the files on disk:\n\n${names(paths)}`
      : `Delete these from disk and schedule them for removal:\n\n${names(paths)}`;
    ask(keepLocal ? "Remove from svn" : "Delete", text, keepLocal ? "Remove" : "Delete", async () => {
      if ((await run("Deleting", svn.Delete(paths, keepLocal))) !== failed) notice = `Scheduled ${paths.length} item(s) for deletion.`;
      await refresh();
    });
  }

  function openMove(path: string) {
    mv = { from: path, to: path };
    modal = "move";
  }

  async function submitMove() {
    if (!mv.to || mv.to === mv.from) return;
    modal = null;
    if ((await run("Moving", svn.Move(mv.from, mv.to))) !== failed) notice = `Moved to ${relative(wc, mv.to)}.`;
    await refresh();
  }

  function contextItems(path: string): MenuItem[] {
    const e = entry(path);
    const paths = targetsFor(path);
    const sel = paths.map(entry).filter((x): x is StatusEntry => !!x);
    const many = paths.length > 1;
    const n = many ? ` (${paths.length})` : "";
    const versioned = !!e && isVersioned(e);
    const unversioned = sel.filter((x) => !isVersioned(x)).map((x) => x.path);
    return [
      { label: "Show diff", action: () => showDiff(path), disabled: many || !versioned },
      { separator: true },
      { label: "Add" + (unversioned.length > 1 ? ` (${unversioned.length})` : ""), action: () => add(unversioned), disabled: !unversioned.length },
      { label: "Revert..." + n, action: () => revert(paths), disabled: !sel.some(isVersioned) },
      { label: "Delete..." + n, action: () => remove(paths, false), disabled: unversioned.length > 0 },
      { label: "Remove from svn, keep file..." + n, action: () => remove(paths, true), disabled: unversioned.length > 0 },
      { label: "Rename / move...", action: () => openMove(path), disabled: many || !versioned },
      { separator: true },
      { label: "Show log", action: () => openLog(path), disabled: many || !versioned || e?.item === "added" },
      { label: "Copy path", action: () => Clipboard.SetText(path) },
    ];
  }

  let commitLabel = $derived(checked.size ? `Commit ${checked.size} selected` : "Commit all changes");
  let canCommit = $derived(!!wc && !busy && !!message.trim() && entries.some((e) => hasChanges(e) || checked.has(e.path)));

  async function commit() {
    if (!canCommit) return;
    const sel = [...checked];
    const toAdd = sel.filter((p) => {
      const e = entry(p);
      return e && !isVersioned(e);
    });
    if (toAdd.length && (await run("Adding new files", svn.Add(toAdd))) === failed) return;
    const rev = await run("Committing", svn.Commit(new CommitRequest({ paths: sel.length ? sel : [wc], message })));
    if (rev === failed) return;
    message = "";
    checked.clear();
    await refresh();
    await reloadInfo();
    notice = rev ? `Committed revision ${rev}.` : "Nothing to commit.";
  }

  async function update() {
    if (!wc || busy) return;
    const rev = await run("Updating", svn.Update([wc]));
    if (rev === failed) return;
    await refresh();
    await reloadInfo();
    notice = `At revision ${rev}.`;
  }

  function openCheckout() {
    co = { url: "", path: "", revision: "", depth: Depth.DepthDefault };
    modal = "checkout";
  }

  async function browseCheckoutDir() {
    const dir = await Dialogs.OpenFile({ Title: "Check out into", CanChooseDirectories: true, CanChooseFiles: false, CanCreateDirectories: true });
    if (!dir) return;
    const name = co.url.replace(/\/+$/, "").split("/").pop() ?? "";
    co.path = name ? join(dir, name) : dir;
  }

  async function submitCheckout() {
    const rev = Number(co.revision || 0);
    if (!co.url || !co.path || !Number.isInteger(rev) || rev < 0) return;
    modal = null;
    const req = new CheckoutRequest({ url: co.url.trim(), path: co.path.trim(), revision: rev, depth: co.depth });
    const r = await run(`Checking out ${req.url}`, svn.Checkout(req));
    if (r === failed) return;
    await openWC(req.path);
    notice = `Checked out revision ${r}.`;
  }

  async function openLog(target: string) {
    log = { target, entries: [], selected: 0 };
    const r = await run("Reading log", svn.Log(new LogRequest({ target, limit: 200, verbose: true })));
    if (r === failed) return;
    log.entries = r;
    modal = "log";
  }

  function openMerge() {
    mg = { source: "^/trunk", change: "", dryRun: true, output: "" };
    modal = "merge";
  }

  async function submitMerge() {
    const change = Number(mg.change || 0);
    if (!mg.source || !Number.isInteger(change) || change < 0) return;
    const out = await run(mg.dryRun ? "Trying merge" : "Merging", svn.Merge(new MergeRequest({ source: mg.source.trim(), target: wc, change, dryRun: mg.dryRun })));
    if (out === failed) return;
    mg.output = out.trim() || "Nothing to merge.";
    if (!mg.dryRun) await refresh();
  }

  function openShelve() {
    sh = { name: "", message: "" };
    modal = "shelve";
  }

  async function submitShelve() {
    if (!sh.name.trim()) return;
    modal = null;
    if ((await run("Shelving", svn.Shelve(wc, sh.name.trim(), sh.message))) !== failed) notice = `Shelved as ${sh.name.trim()}.`;
    await refresh();
  }

  async function openUnshelve() {
    const r = await run("Reading shelves", svn.Shelves(wc));
    if (r === failed) return;
    shelves = r;
    modal = "unshelve";
  }

  async function unshelve(name: string) {
    modal = null;
    if ((await run("Unshelving", svn.Unshelve(wc, name))) !== failed) notice = `Unshelved ${name}.`;
    await refresh();
  }

  async function openHelp(topic = "") {
    const r = await run("Reading help", svn.Help(topic));
    if (r === failed) return;
    help = { topic, text: r };
    modal = "help";
  }

  async function openAbout() {
    modal = "about";
    if (!appInfo) {
      try {
        appInfo = await AppService.About();
      } catch {}
    }
  }

  // Check only starts the updater; Wails opens its own update window with the progress and the result.
  async function checkUpdates() {
    updateNote = "Checking...";
    try {
      await UpdateService.Check();
      updateNote = "The update window shows the result.";
    } catch (e) {
      updateNote = errorText(e);
    }
  }

  function showError() {
    if (!errorFull) return;
    textBox = { title: "Error", text: errorFull };
    modal = "text";
  }

  let menus = $derived([
    {
      title: "File",
      items: [
        { label: "Open working copy...", action: openFolder, hint: "Ctrl+O", disabled: !!busy },
        { label: "Checkout...", action: openCheckout, disabled: !!busy },
        { separator: true as const },
        { label: "Quit", action: () => Application.Quit() },
      ],
    },
    {
      title: "Repository",
      items: [
        { label: "Refresh", action: refresh, hint: "F5", disabled: !wc || !!busy },
        { label: "Update", action: update, hint: "Ctrl+U", disabled: !wc || !!busy },
        { label: "Commit", action: commit, hint: "Ctrl+Enter", disabled: !canCommit },
        { separator: true as const },
        { label: "Show log", action: () => openLog(wc), disabled: !wc || !!busy },
        { label: "Merge...", action: openMerge, disabled: !wc || !!busy },
        { separator: true as const },
        { label: "Shelve changes...", action: openShelve, disabled: !wc || !!busy },
        { label: "Unshelve...", action: openUnshelve, disabled: !wc || !!busy },
      ],
    },
    {
      title: "Help",
      items: [
        { label: "svn command reference", action: () => openHelp() },
        { label: "svnch on GitHub", action: () => Browser.OpenURL(REPO_URL) },
      ],
    },
    { title: "Settings", action: () => { updateNote = ""; modal = "settings"; } },
    { title: "About", action: openAbout },
  ]);

  function onKey(e: KeyboardEvent) {
    if (modal || ctxMenu) return;
    const mod = e.ctrlKey || e.metaKey;
    if (e.key === "F5") {
      e.preventDefault();
      refresh();
    } else if (mod && e.key === "o") {
      e.preventDefault();
      openFolder();
    } else if (mod && e.key === "u") {
      e.preventDefault();
      update();
    } else if (mod && e.key === "Enter") {
      e.preventDefault();
      commit();
    }
  }

  onMount(() => {
    let last = "";
    try {
      last = localStorage.getItem(LAST_WC) ?? "";
    } catch {}
    if (last) openWC(last);
  });
</script>

<svelte:window onkeydown={onKey} oncontextmenu={(e) => e.preventDefault()} />

<div class="app">
  <MenuBar {menus} />

  <div class="toolbar">
    <button onclick={openFolder} disabled={!!busy}>Open...</button>
    <button onclick={update} disabled={!wc || !!busy}>Update</button>
    <button onclick={refresh} disabled={!wc || !!busy}>Refresh</button>
    <button onclick={() => openLog(wc)} disabled={!wc || !!busy}>Log</button>
    {#if info}
      <div class="where">
        <span class="path" title={wc}>{wc}</span>
        <span class="muted" title={info.url}>{info.relativeUrl || info.url}</span>
        <span class="rev">r{info.revision}</span>
      </div>
    {/if}
  </div>

  {#if !wc}
    <div class="welcome">
      <img src="/appicon.png" alt="" />
      <p>Open a working copy, or check one out from a repository.</p>
      <div class="actions">
        <button class="primary" onclick={openFolder} disabled={!!busy}>Open working copy...</button>
        <button onclick={openCheckout} disabled={!!busy}>Checkout...</button>
      </div>
    </div>
  {:else}
    <main>
      <section class="left">
        <div class="files">
          <FileList
            {wc}
            {entries}
            {checked}
            {focused}
            onfocus={showDiff}
            ontoggle={(p) => (checked.has(p) ? checked.delete(p) : checked.add(p))}
            ontoggleall={(on) => (on ? entries.forEach((e) => checked.add(e.path)) : checked.clear())}
            oncontext={(p, e) => (ctxMenu = { x: e.clientX, y: e.clientY, items: contextItems(p) })}
          />
        </div>
        <form class="commit" onsubmit={(e) => { e.preventDefault(); commit(); }}>
          <textarea bind:value={message} placeholder="Commit message" rows="4"></textarea>
          <button class="primary" type="submit" disabled={!canCommit}>{commitLabel}</button>
        </form>
      </section>
      <section class="right">
        <DiffView title={diffTitle} text={diffText} loading={diffLoading} />
      </section>
    </main>
  {/if}

  <footer class="status">
    {#if busy}
      <span>{busy}...</span>
      {#if cancelBusy}<button class="small" onclick={cancelBusy}>Cancel</button>{/if}
    {:else if error}
      <button class="link error" title="Show the full message" onclick={showError}>{error}</button>
    {:else}
      <span>{notice || (wc ? "Ready." : "")}</span>
    {/if}
  </footer>
</div>

{#if ctxMenu}
  <ContextMenu x={ctxMenu.x} y={ctxMenu.y} items={ctxMenu.items} onclose={() => (ctxMenu = null)} />
{/if}

{#if modal === "checkout"}
  <Modal title="Checkout" onclose={() => (modal = null)}>
    <label><span>Repository URL</span><input bind:value={co.url} placeholder="https://svn.example.org/repo/trunk" /></label>
    <label>
      <span>Into folder</span>
      <div class="row"><input bind:value={co.path} placeholder="/home/me/work/trunk" /><button type="button" onclick={browseCheckoutDir}>Browse...</button></div>
    </label>
    <div class="row">
      <label class="grow"><span>Revision (empty for HEAD)</span><input bind:value={co.revision} inputmode="numeric" /></label>
      <label class="grow">
        <span>Depth</span>
        <select bind:value={co.depth}>
          <option value={Depth.DepthDefault}>everything</option>
          <option value={Depth.DepthImmediates}>immediates</option>
          <option value={Depth.DepthFiles}>files only</option>
          <option value={Depth.DepthEmpty}>empty</option>
        </select>
      </label>
    </div>
    {#snippet footer()}
      <button onclick={() => (modal = null)}>Cancel</button>
      <button class="primary" disabled={!co.url || !co.path} onclick={submitCheckout}>Checkout</button>
    {/snippet}
  </Modal>
{:else if modal === "move"}
  <Modal title="Rename / move" onclose={() => (modal = null)}>
    <label><span>From</span><input value={mv.from} readonly /></label>
    <label><span>To</span><input bind:value={mv.to} /></label>
    {#snippet footer()}
      <button onclick={() => (modal = null)}>Cancel</button>
      <button class="primary" disabled={!mv.to || mv.to === mv.from} onclick={submitMove}>Move</button>
    {/snippet}
  </Modal>
{:else if modal === "confirm"}
  <Modal title={confirm.title} onclose={() => (modal = null)}>
    <pre class="note">{confirm.text}</pre>
    {#snippet footer()}
      <button onclick={() => (modal = null)}>Cancel</button>
      <button class="primary" onclick={() => { modal = null; confirm.run(); }}>{confirm.ok}</button>
    {/snippet}
  </Modal>
{:else if modal === "merge"}
  <Modal title="Merge into working copy" wide onclose={() => (modal = null)}>
    <div class="row">
      <label class="grow"><span>From (URL or ^/path)</span><input bind:value={mg.source} /></label>
      <label><span>Only revision (empty for all)</span><input bind:value={mg.change} inputmode="numeric" /></label>
    </div>
    <label class="check"><input type="checkbox" bind:checked={mg.dryRun} /> Dry run, change nothing</label>
    {#if mg.output}<pre class="output">{mg.output}</pre>{/if}
    {#snippet footer()}
      <button onclick={() => (modal = null)}>Close</button>
      <button class="primary" disabled={!mg.source || !!busy} onclick={submitMerge}>{mg.dryRun ? "Try merge" : "Merge"}</button>
    {/snippet}
  </Modal>
{:else if modal === "shelve"}
  <Modal title="Shelve changes" onclose={() => (modal = null)}>
    <p class="muted">Puts every local change aside and reverts the working copy. Needs svn 1.14 or newer.</p>
    <label><span>Name</span><input bind:value={sh.name} placeholder="half-done-feature" /></label>
    <label><span>Note (optional)</span><input bind:value={sh.message} /></label>
    {#snippet footer()}
      <button onclick={() => (modal = null)}>Cancel</button>
      <button class="primary" disabled={!sh.name.trim()} onclick={submitShelve}>Shelve</button>
    {/snippet}
  </Modal>
{:else if modal === "unshelve"}
  <Modal title="Unshelve" onclose={() => (modal = null)}>
    {#each shelves as name}
      <div class="row between"><span>{name}</span><button onclick={() => unshelve(name)}>Unshelve</button></div>
    {:else}
      <p class="muted">No shelves in this working copy.</p>
    {/each}
  </Modal>
{:else if modal === "log"}
  <Modal title="Log: {relative(wc, log.target)}" wide onclose={() => (modal = null)}>
    <div class="log">
      <ul class="revs">
        {#each log.entries as e, i}
          <li>
            <button class:sel={i === log.selected} onclick={() => (log.selected = i)}>
              <b>r{e.revision}</b> <span class="muted">{e.author}</span>
              <span class="msg">{e.message.split("\n")[0] || "(no message)"}</span>
            </button>
          </li>
        {:else}
          <li class="muted">No revisions.</li>
        {/each}
      </ul>
      {#if log.entries[log.selected]}
        {@const e = log.entries[log.selected]}
        <div class="detail">
          <div><b>r{e.revision}</b> by {e.author || "unknown"}, {new Date(e.date).toLocaleString()}</div>
          <pre class="output">{e.message || "(no message)"}</pre>
          <pre class="paths">{#each e.paths ?? [] as p}<span class="act {p.action}">{p.action}</span> {p.path}{"\n"}{/each}</pre>
        </div>
      {/if}
    </div>
    {#snippet footer()}
      <button onclick={() => (modal = null)}>Close</button>
      <button
        class="primary"
        disabled={!log.entries[log.selected]}
        onclick={() => { const e = log.entries[log.selected]; modal = null; showRevisionDiff(log.target, e.revision); }}
      >Show diff</button>
    {/snippet}
  </Modal>
{:else if modal === "help"}
  <Modal title="svn help" wide onclose={() => (modal = null)}>
    <form class="row" onsubmit={(e) => { e.preventDefault(); openHelp(help.topic.trim()); }}>
      <input class="grow" bind:value={help.topic} placeholder="command, e.g. merge" />
      <button type="submit">Show</button>
    </form>
    <pre class="output">{help.text}</pre>
  </Modal>
{:else if modal === "text"}
  <Modal title={textBox.title} wide onclose={() => (modal = null)}>
    <pre class="output">{textBox.text}</pre>
  </Modal>
{:else if modal === "about"}
  <Modal title="About svnch" onclose={() => (modal = null)}>
    <div class="about">
      <img src="/appicon.png" alt="svnch" />
      <div>
        <h1>svnch {#if appInfo?.version}<span class="ver">{appInfo.version}</span>{/if}</h1>
        <p>A small desktop client for Subversion.<br />It runs the svn commands you would type yourself.</p>
        <p class="muted">{appInfo?.copyright || "(c) 2026 Canercan Demir, MIT License"}</p>
        {#if appInfo}
          <p class="muted">{appInfo.platform} · {appInfo.go}{appInfo.wails ? ` · Wails ${appInfo.wails}` : ""}</p>
        {/if}
        <button onclick={() => Browser.OpenURL(REPO_URL)}>github.com/Cn8001/svnch</button>
      </div>
    </div>
  </Modal>
{:else if modal === "settings"}
  <Modal title="Settings" onclose={() => (modal = null)}>
    <div class="setting">
      <div>
        <b>Updates</b>
        <p class="muted">svnch looks for a new release each time it starts.</p>
      </div>
      <button onclick={checkUpdates}>Check for updates</button>
    </div>
    {#if updateNote}<p>{updateNote}</p>{/if}
  </Modal>
{/if}

<style>
  .app { display: grid; grid-template-rows: auto auto 1fr auto; height: 100%; }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 8px;
    border-bottom: 2px solid var(--ink);
  }
  .where { display: flex; align-items: baseline; gap: 12px; margin-left: 8px; min-width: 0; }
  .where span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .where .path { font-weight: bold; }
  .rev { padding: 0 6px; border: 2px solid var(--ink); background: var(--cream); }

  .welcome { display: grid; place-content: center; justify-items: center; gap: 12px; text-align: center; }
  .welcome img { width: 128px; height: 128px; }
  .welcome p { margin: 0; }
  .actions { display: flex; gap: 10px; }

  main { display: grid; grid-template-columns: minmax(280px, 38%) 1fr; min-height: 0; }
  .left { display: flex; flex-direction: column; min-height: 0; border-right: 2px solid var(--ink); background: var(--panel); }
  .files { flex: 1; min-height: 0; }
  .commit { display: grid; gap: 6px; padding: 8px; border-top: 2px solid var(--ink); background: var(--cream); }
  .right { min-width: 0; min-height: 0; }

  .status {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 26px;
    padding: 2px 8px;
    border-top: 2px solid var(--ink);
    background: var(--cream);
  }
  .status span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .small { padding: 0 8px; box-shadow: 1px 1px 0 var(--ink); }
  .link { border: 0; box-shadow: none; background: transparent; padding: 0; text-align: left; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .link:hover:not(:disabled) { background: transparent; text-decoration: underline; }

  .row { display: flex; gap: 8px; align-items: end; }
  .row > input { flex: 1; }
  .between { justify-content: space-between; align-items: center; }
  .grow { flex: 1; }
  .check { display: flex; align-items: center; gap: 6px; }
  .note { white-space: pre-wrap; }
  .output {
    max-height: 50vh;
    overflow: auto;
    padding: 8px;
    background: var(--field);
    border: 2px solid var(--ink);
    white-space: pre-wrap;
  }

  .log { display: grid; grid-template-columns: minmax(240px, 40%) 1fr; gap: 10px; height: 55vh; }
  .revs { margin: 0; padding: 0; list-style: none; overflow: auto; border: 2px solid var(--ink); background: var(--field); }
  .revs button {
    display: block;
    width: 100%;
    border: 0;
    border-bottom: 1px dotted rgba(36, 33, 30, 0.2);
    box-shadow: none;
    background: transparent;
    text-align: left;
    padding: 4px 8px;
  }
  .revs button:active { transform: none; }
  .revs button.sel { background: var(--select); }
  .revs .msg { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .detail { display: grid; grid-template-rows: auto auto 1fr; gap: 8px; min-height: 0; }
  .paths { overflow: auto; padding: 8px; border: 2px solid var(--ink); background: var(--field); }
  .act { font-weight: bold; }
  .act.A { color: var(--add-ink); }
  .act.D { color: var(--brick); }

  .about { display: flex; gap: 16px; align-items: center; }
  .about img { width: 112px; height: 112px; }
  .about h1 { margin: 0 0 6px; font-size: 20px; }
  .about p { margin: 0 0 10px; }
  .ver { font-weight: normal; color: var(--muted); }
  .setting { display: flex; justify-content: space-between; align-items: center; gap: 16px; }
  .setting p { margin: 4px 0 0; }
</style>
