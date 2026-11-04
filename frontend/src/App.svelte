<script lang="ts">
  import { onMount } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import { Application, Browser, Clipboard, Dialogs } from "@wailsio/runtime";
  import MenuBar from "./lib/MenuBar.svelte";
  import FileList from "./lib/FileList.svelte";
  import DiffView from "./lib/DiffView.svelte";
  import ContextMenu from "./lib/ContextMenu.svelte";
  import Modal from "./lib/Modal.svelte";
  import SvnMissing from "./lib/SvnMissing.svelte";
  import RepoBrowser from "./lib/RepoBrowser.svelte";
  import {
    svn,
    AppService,
    UpdateService,
    SetupService,
    ServerService,
    ExplorerService,
    failureCode,
    type ExplorerStatus,
    type Server,
    type Setup,
    type AppInfo,
    type CheckoutRequest,
    Depth,
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
  const THEME = "svnch.theme";

  type Theme = "dark" | "light" | "system";
  let theme = $state<Theme>(readTheme());
  const prefersLight = matchMedia("(prefers-color-scheme: light)");
  let systemLight = $state(prefersLight.matches);
  prefersLight.addEventListener("change", (e) => (systemLight = e.matches));

  function readTheme(): Theme {
    try {
      const t = localStorage.getItem(THEME);
      return t === "light" || t === "system" ? t : "dark";
    } catch {
      return "dark";
    }
  }

  function setTheme(t: Theme) {
    theme = t;
    try {
      localStorage.setItem(THEME, t);
    } catch {}
  }

  $effect(() => {
    const light = theme === "light" || (theme === "system" && systemLight);
    document.documentElement.dataset.theme = light ? "light" : "dark";
  });

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

  type ModalKind = "checkout" | "move" | "confirm" | "merge" | "shelve" | "unshelve" | "log" | "help" | "text" | "about" | "settings" | "licenses" | "repos" | "svn";

  let setup = $state<Setup | null>(null);
  let checkingSvn = $state(false);
  let servers: Server[] = $state([]);
  let login = $state({ url: "", username: "", password: "", error: "", busy: false, manual: false }); // manual: opened by the user, so the url is editable
  let sessionUser = $state("");
  let loginDone: ((ok: boolean) => void) | null = null;
  let loginOpen = $state(false); // over whatever window asked, so a half-filled form there survives the login

  let appInfo: AppInfo | null = $state(null);
  let updateNote = $state("");
  let explorerMenu = $state<ExplorerStatus | null>(null);
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

  // Every svn call goes through here so the status bar, the cancel button, error reporting and the login prompt behave
  // the same. make is called again after a login, so a command that hit a password-protected server just carries on.
  async function run<T>(label: string, make: () => Promise<T> & { cancel?: () => void }, url = ""): Promise<T | typeof failed> {
    for (let attempt = 0; ; attempt++) {
      let cancelled = false;
      const p = make();
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
        busy = "";
        cancelBusy = null;
        const code = cancelled ? "CANCELLED" : failureCode(e);
        const where = url || info?.repositoryRoot || "";
        if (code === "AUTH" && attempt < 3 && where && (await askLogin(where, attempt ? "That did not work, try again." : ""))) continue;
        if (code === "CERT") {
          textBox = {
            title: "Untrusted server certificate",
            text: `${errorText(e)}\n\nsvn does not trust this server's HTTPS certificate. Check it once in a terminal and accept it permanently:\n\n  svn info ${where || "<url>"}\n\nthen press (p) when svn asks.`,
          };
          modal = "text";
        }
        if (code === "SVN_MISSING") await checkSvn();
        error = cancelled ? "Cancelled." : errorText(e);
        errorFull = cancelled ? "" : e instanceof Error ? e.message : String(e);
        return failed;
      } finally {
        busy = "";
        cancelBusy = null;
      }
    }
  }

  function askLogin(url: string, message = "", manual = false): Promise<boolean> {
    const known = servers.find((s) => url.startsWith(s.url));
    login = { url, username: known?.username || login.username, password: "", error: message, busy: false, manual };
    loginOpen = true;
    return new Promise((resolve) => (loginDone = resolve));
  }

  function finishLogin(ok: boolean) {
    loginOpen = false;
    loginDone?.(ok);
    loginDone = null;
  }

  function startLogin(url = "") {
    askLogin(url || info?.repositoryRoot || servers[0]?.url || "", "", true);
  }

  async function submitLogin() {
    login.url = login.url.trim().replace(/\/+$/, "");
    if (!login.username || !login.url) return;
    login.busy = true;
    login.error = "";
    try {
      await svn.Login(login.url, login.username, login.password);
    } catch (e) {
      login.busy = false;
      login.error = failureCode(e) === "AUTH" ? "Wrong username or password." : errorText(e);
      return;
    }
    login.busy = false;
    sessionUser = login.username;
    const known = servers.find((s) => login.url.startsWith(s.url));
    if (known && known.username !== login.username) {
      try {
        await ServerService.Save({ ...known, username: login.username });
        await loadServers();
      } catch {}
    }
    finishLogin(true);
  }

  async function loadServers() {
    try {
      servers = (await ServerService.List()) ?? [];
    } catch {}
  }

  async function checkSvn() {
    checkingSvn = true;
    try {
      setup = await SetupService.Status();
    } catch {}
    checkingSvn = false;
  }

  let svnMissing = $derived(!!setup && !setup.binary);

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
    const i = await run(`Reading ${dir}`, () => svn.Info(dir));
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
    const r = await run("Reading status", () => svn.Status(wc));
    if (r === failed) return;
    entries = (r ?? []).sort((a, b) => a.path.localeCompare(b.path));
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
      const text = await svn.Diff({ target: path, change: 0 });
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
      const text = await svn.Diff({ target, change: rev });
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
    if ((await run("Adding", () => svn.Add(paths))) !== failed) notice = `Added ${paths.length} item(s).`;
    await refresh();
  }

  function revert(paths: string[]) {
    ask("Revert", `Throw away local changes in:\n\n${names(paths)}`, "Revert", async () => {
      if ((await run("Reverting", () => svn.Revert(paths))) !== failed) notice = `Reverted ${paths.length} item(s).`;
      await refresh();
    });
  }

  function remove(paths: string[], keepLocal: boolean) {
    const text = keepLocal
      ? `Stop versioning these, but keep the files on disk:\n\n${names(paths)}`
      : `Delete these from disk and schedule them for removal:\n\n${names(paths)}`;
    ask(keepLocal ? "Remove from svn" : "Delete", text, keepLocal ? "Remove" : "Delete", async () => {
      if ((await run("Deleting", () => svn.Delete(paths, keepLocal))) !== failed) notice = `Scheduled ${paths.length} item(s) for deletion.`;
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
    if ((await run("Moving", () => svn.Move(mv.from, mv.to))) !== failed) notice = `Moved to ${relative(wc, mv.to)}.`;
    await refresh();
  }

  function contextItems(path: string): MenuItem[] {
    const e = entry(path);
    const paths = targetsFor(path);
    const sel = paths.map(entry).filter((x): x is StatusEntry => !!x);
    const many = paths.length > 1;
    const n = many ? ` (${paths.length})` : "";
    const folder = !e; // a folder row in the tree that only holds changes; svn lists it only when it changed itself
    const versioned = folder || isVersioned(e);
    const unversioned = sel.filter((x) => !isVersioned(x)).map((x) => x.path);
    return [
      { label: "Show diff", action: () => showDiff(path), disabled: many || !versioned },
      { separator: true },
      { label: "Add" + (unversioned.length > 1 ? ` (${unversioned.length})` : ""), action: () => add(unversioned), disabled: !unversioned.length },
      { label: "Revert..." + n, action: () => revert(paths), disabled: !folder && !sel.some(isVersioned) },
      { label: "Delete..." + n, action: () => remove(paths, false), disabled: unversioned.length > 0 },
      { label: "Remove from svn, keep file..." + n, action: () => remove(paths, true), disabled: unversioned.length > 0 },
      { label: "Rename / move...", action: () => openMove(path), disabled: many || !versioned },
      { separator: true },
      { label: "Show log", action: () => openLog(path), disabled: many || !versioned || e?.item === "added" || e?.item === "ignored" },
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
    if (toAdd.length && (await run("Adding new files", () => svn.Add(toAdd))) === failed) return;
    const rev = await run("Committing", () => svn.Commit({ paths: sel.length ? sel : [wc], message }));
    if (rev === failed) return;
    message = "";
    checked.clear();
    await refresh();
    await reloadInfo();
    notice = rev ? `Committed revision ${rev}.` : "Nothing to commit.";
  }

  async function update() {
    if (!wc || busy) return;
    const rev = await run("Updating", () => svn.Update([wc]));
    if (rev === failed) return;
    await refresh();
    await reloadInfo();
    notice = `At revision ${rev}.`;
  }

  function openCheckout(url = "", path = "") {
    co = { url, path, revision: "", depth: Depth.DepthDefault };
    modal = "checkout";
  }

  async function openRepos() {
    await loadServers();
    modal = "repos";
  }

  async function listRepo(url: string) {
    const r = await run(`Listing ${url}`, () => svn.List(url), url);
    return r === failed ? null : (r ?? []);
  }

  // The check goes through run, so a server that wants a password brings up the login with the form's username.
  async function checkServer(url: string, username: string) {
    const target = url.trim().replace(/\/+$/, "");
    if (username) login.username = username;
    const r = await run(`Connecting to ${target}`, () => svn.List(target), target);
    if (r === failed) return { ok: false, text: error || "Could not connect.", username: "" };
    const n = (r ?? []).length;
    return {
      ok: true,
      text: `Connected. ${n} ${n === 1 ? "entry" : "entries"} at this address.`,
      username: login.url.startsWith(target) ? login.username : "",
    };
  }

  async function saveServer(s: Server) {
    try {
      await ServerService.Save(s);
      await loadServers();
      return true;
    } catch (e) {
      error = errorText(e);
      return false;
    }
  }

  async function removeServer(url: string) {
    try {
      await ServerService.Remove(url);
    } catch (e) {
      error = errorText(e);
    }
    await loadServers();
  }

  async function forgetLogin() {
    await svn.Logout();
    sessionUser = "";
    updateNote = "Forgotten for this session. svn may still have the password saved in its own store.";
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
    const req: CheckoutRequest = { url: co.url.trim(), path: co.path.trim(), revision: rev, depth: co.depth };
    const r = await run(`Checking out ${req.url}`, () => svn.Checkout(req), req.url);
    if (r === failed) return;
    await openWC(req.path);
    notice = `Checked out revision ${r}.`;
  }

  async function openLog(target: string) {
    log = { target, entries: [], selected: 0 };
    const r = await run("Reading log", () => svn.Log({ target, limit: 200, verbose: true }), target);
    if (r === failed) return;
    log.entries = r ?? [];
    modal = "log";
  }

  function openMerge() {
    mg = { source: "^/trunk", change: "", dryRun: true, output: "" };
    modal = "merge";
  }

  async function submitMerge() {
    const change = Number(mg.change || 0);
    if (!mg.source || !Number.isInteger(change) || change < 0) return;
    const out = await run(mg.dryRun ? "Trying merge" : "Merging", () => svn.Merge({ source: mg.source.trim(), target: wc, change, dryRun: mg.dryRun }));
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
    if ((await run("Shelving", () => svn.Shelve(wc, sh.name.trim(), sh.message))) !== failed) notice = `Shelved as ${sh.name.trim()}.`;
    await refresh();
  }

  async function openUnshelve() {
    const r = await run("Reading shelves", () => svn.Shelves(wc));
    if (r === failed) return;
    shelves = r ?? [];
    modal = "unshelve";
  }

  async function unshelve(name: string) {
    modal = null;
    if ((await run("Unshelving", () => svn.Unshelve(wc, name))) !== failed) notice = `Unshelved ${name}.`;
    await refresh();
  }

  async function openHelp(topic = "") {
    const r = await run("Reading help", () => svn.Help(topic));
    if (r === failed) return;
    help = { topic, text: r };
    modal = "help";
  }

  // A folder from Explorer's "Open with svnch" may not be a working copy yet; then it is where a checkout goes.
  async function openStart(dir: string) {
    try {
      await svn.Info(dir);
    } catch (e) {
      if (failureCode(e) === "NOT_WORKING_COPY") {
        openCheckout("", dir);
        return;
      }
    }
    await openWC(dir);
  }

  async function openSettings() {
    updateNote = "";
    modal = "settings";
    try {
      sessionUser = await svn.LoggedInAs();
    } catch {}
    try {
      explorerMenu = await ExplorerService.Status();
    } catch {}
  }

  async function toggleExplorer() {
    if (!explorerMenu) return;
    try {
      await ExplorerService.SetEnabled(!explorerMenu.enabled);
      explorerMenu = await ExplorerService.Status();
    } catch (e) {
      updateNote = errorText(e);
    }
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
        { label: "Checkout...", action: () => openCheckout(), disabled: !!busy || svnMissing },
        { label: "Browse repositories...", action: openRepos, hint: "Ctrl+B", disabled: !!busy || svnMissing },
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
        { label: "Installing svn", action: () => { checkSvn(); modal = "svn"; } },
        { label: "svnch on GitHub", action: () => Browser.OpenURL(REPO_URL) },
      ],
    },
    { title: "Settings", action: openSettings },
    { title: "About", action: openAbout },
  ]);

  function onKey(e: KeyboardEvent) {
    if (modal || loginOpen || ctxMenu) return;
    const mod = e.ctrlKey || e.metaKey;
    if (e.key === "F5") {
      e.preventDefault();
      refresh();
    } else if (mod && e.key === "o") {
      e.preventDefault();
      openFolder();
    } else if (mod && e.key === "b") {
      e.preventDefault();
      if (!svnMissing) openRepos();
    } else if (mod && e.key === "u") {
      e.preventDefault();
      update();
    } else if (mod && e.key === "Enter") {
      e.preventDefault();
      commit();
    }
  }

  onMount(async () => {
    loadServers();
    await checkSvn();
    if (svnMissing) return;
    let start = "";
    try {
      start = await AppService.StartFolder();
    } catch {}
    if (start) return openStart(start);
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
    <button onclick={openFolder} disabled={!!busy || svnMissing}>Open...</button>
    <button onclick={openRepos} disabled={!!busy || svnMissing}>Repos</button>
    <span class="sep"></span>
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

  {#if svnMissing}
    <div class="welcome">
      <img src="/appicon.png" alt="" />
      <SvnMissing {setup} checking={checkingSvn} oncheck={checkSvn} />
    </div>
  {:else if !wc}
    <div class="welcome">
      <img src="/appicon.png" alt="" />
      <p>Open a working copy, or check one out from a repository.</p>
      <div class="actions">
        <button class="primary" onclick={openFolder} disabled={!!busy}>Open working copy...</button>
        <button onclick={() => openCheckout()} disabled={!!busy}>Checkout...</button>
        <button onclick={openRepos} disabled={!!busy}>Browse repositories...</button>
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
    <label>
      <span>Repository URL</span>
      <div class="row">
        <input bind:value={co.url} list="saved-servers" placeholder="https://svn.example.org/repo/trunk" />
        <button type="button" onclick={openRepos}>Browse...</button>
      </div>
    </label>
    <datalist id="saved-servers">
      {#each servers as s (s.url)}<option value={s.url}>{s.name}</option>{/each}
    </datalist>
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
        <div class="row">
          <button onclick={() => Browser.OpenURL(REPO_URL)}>github.com/Cn8001/svnch</button>
          <button onclick={() => (modal = "licenses")} disabled={!appInfo}>Third-party licences</button>
        </div>
      </div>
    </div>
  </Modal>
{:else if modal === "licenses"}
  <Modal title="Third-party licences" wide onclose={() => (modal = "about")}>
    {#if appInfo?.notices}<pre class="output notices">{appInfo.notices}</pre>{/if}
    <div class="licenses">
      {#each appInfo?.licenses ?? [] as l (l.name + l.version)}
        <details>
          <summary><b>{l.name}</b> <span class="muted">{l.version}</span> <span class="lic">{l.license || "see text"}</span></summary>
          <pre class="output">{l.text}</pre>
          {#if l.notice}<pre class="output">{l.notice}</pre>{/if}
        </details>
      {/each}
    </div>
    {#snippet footer()}
      <button onclick={() => (modal = "about")}>Back</button>
    {/snippet}
  </Modal>
{:else if modal === "settings"}
  <Modal title="Settings" onclose={() => (modal = null)}>
    <div class="setting">
      <div>
        <b>Theme</b>
        <p class="muted">System follows your desktop's light or dark setting.</p>
      </div>
      <div class="segmented" role="group" aria-label="Theme">
        {#each [["dark", "Dark"], ["light", "Light"], ["system", "System"]] as [value, label]}
          <button class:on={theme === value} onclick={() => setTheme(value as Theme)}>{label}</button>
        {/each}
      </div>
    </div>
    <div class="setting">
      <div>
        <b>Updates</b>
        <p class="muted">svnch looks for a new release each time it starts.</p>
      </div>
      <button onclick={checkUpdates}>Check for updates</button>
    </div>
    <div class="setting">
      <div>
        <b>Login</b>
        <p class="muted">
          {sessionUser ? `Logged in as ${sessionUser} for this session.` : "Not logged in this session; svn may still use a password it saved."}
        </p>
      </div>
      <div class="buttons">
        <button onclick={() => startLogin()}>{sessionUser ? "Switch user..." : "Log in..."}</button>
        <button onclick={forgetLogin}>Forget login</button>
      </div>
    </div>
    {#if explorerMenu?.supported}
      <div class="setting">
        <div>
          <b>Explorer</b>
          <p class="muted">"Open with svnch" when you right-click a folder. On Windows 11 it is under "Show more options".</p>
        </div>
        <button onclick={toggleExplorer}>{explorerMenu.enabled ? "Remove from menu" : "Add to menu"}</button>
      </div>
    {/if}
    {#if updateNote}<p>{updateNote}</p>{/if}
  </Modal>
{:else if modal === "repos"}
  <Modal title="Repositories" wide onclose={() => (modal = null)}>
    <RepoBrowser
      {servers}
      list={listRepo}
      onsave={saveServer}
      onremove={removeServer}
      oncheckout={(url) => openCheckout(url)}
      onlog={(url) => openLog(url)}
      check={checkServer}
      onlogin={startLogin}
      cancel={() => cancelBusy?.()}
    />
  </Modal>
{:else if modal === "svn"}
  <Modal title="Installing svn" onclose={() => (modal = null)}>
    {#if setup?.binary}
      <p>svn {setup.version} is installed at <code>{setup.binary}</code>.</p>
    {:else}
      <SvnMissing {setup} checking={checkingSvn} oncheck={checkSvn} />
    {/if}
  </Modal>
{/if}

{#if loginOpen}
  <Modal title="Login" onclose={() => finishLogin(false)}>
    <form class="login" onsubmit={(e) => { e.preventDefault(); submitLogin(); }}>
      {#if login.manual}
        <p>Log in to a server for this session.</p>
        <label><span>Server</span><input bind:value={login.url} list="login-servers" placeholder="https://svn.example.org/svn" /></label>
        <datalist id="login-servers">
          {#each servers as s (s.url)}<option value={s.url}>{s.name}</option>{/each}
        </datalist>
      {:else}
        <p>The server asks who you are.</p>
        <p class="muted where" title={login.url}>{login.url}</p>
      {/if}
      <!-- svelte-ignore a11y_autofocus -->
      <label><span>Username</span><input bind:value={login.username} autofocus={!login.username} autocomplete="username" /></label>
      <!-- svelte-ignore a11y_autofocus -->
      <label><span>Password</span><input type="password" bind:value={login.password} autofocus={!!login.username} autocomplete="current-password" /></label>
      {#if login.error}<p class="error">{login.error}</p>{/if}
      <p class="muted fine">svn saves the password in your system keyring when it can. Where it cannot, svnch keeps it until you close the app.</p>
      <div class="end">
        <button type="button" onclick={() => finishLogin(false)}>Cancel</button>
        <button class="primary" type="submit" disabled={!login.username || !login.url.trim() || login.busy}>{login.busy ? "Checking..." : "Log in"}</button>
      </div>
    </form>
  </Modal>
{/if}

<style>
  .app { display: grid; grid-template-rows: auto auto 1fr auto; height: 100%; }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 10px;
    background: var(--surface);
    border-bottom: 1px solid var(--line);
  }
  .toolbar .sep { width: 1px; align-self: stretch; margin: 4px 4px; background: var(--line); }
  .where { display: flex; align-items: center; gap: 10px; margin-left: auto; min-width: 0; }
  .where span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .where .path { font-family: var(--mono); font-size: 12px; font-weight: 600; }
  .where .muted { font-family: var(--mono); font-size: 12px; }
  .rev { padding: 1px 8px; border-radius: 10px; background: var(--blue-soft); color: var(--blue); font-family: var(--mono); font-size: 12px; font-weight: 700; }

  .welcome { display: grid; place-content: center; justify-items: center; gap: 14px; text-align: center; }
  .welcome img { width: 120px; height: 120px; }
  .welcome p { margin: 0; }
  .actions { display: flex; gap: 10px; }

  main { display: grid; grid-template-columns: minmax(300px, 36%) 1fr; min-height: 0; }
  .left { display: flex; flex-direction: column; min-height: 0; border-right: 1px solid var(--line); background: var(--surface); }
  .files { flex: 1; min-height: 0; }
  .commit { display: grid; gap: 8px; padding: 10px; border-top: 1px solid var(--line); }
  .commit button { padding: 6px 12px; }
  .right { min-width: 0; min-height: 0; background: var(--bg); }

  .status {
    display: flex;
    align-items: center;
    gap: 10px;
    min-height: 26px;
    padding: 2px 10px;
    border-top: 1px solid var(--line);
    background: var(--surface);
    font-size: 12px;
    color: var(--muted);
  }
  .status span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .small { padding: 0 8px; font-size: 12px; }
  .link { border: 0; background: transparent; padding: 0; text-align: left; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
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
    padding: 10px;
    background: var(--field);
    border: 1px solid var(--line);
    border-radius: var(--radius);
    white-space: pre-wrap;
  }

  .log { display: grid; grid-template-columns: minmax(240px, 40%) 1fr; gap: 12px; height: 55vh; }
  .revs { margin: 0; padding: 4px; list-style: none; overflow: auto; border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); }
  .revs button {
    display: block;
    width: 100%;
    border: 0;
    border-radius: 4px;
    background: transparent;
    text-align: left;
    padding: 5px 8px;
  }
  .revs button:hover:not(:disabled) { background: var(--hover); }
  .revs button.sel { background: var(--accent-soft); box-shadow: inset 2px 0 0 var(--accent); }
  .revs .msg { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .detail { display: grid; grid-template-rows: auto auto 1fr; gap: 8px; min-height: 0; }
  .paths { overflow: auto; padding: 10px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); }
  .act { font-weight: bold; }
  .act.A { color: var(--add-ink); }
  .act.M { color: var(--mod-ink); }
  .act.D { color: var(--del-ink); }

  .about { display: flex; gap: 18px; align-items: center; }
  .about img { width: 104px; height: 104px; }
  .about h1 { margin: 0 0 6px; font-size: 20px; }
  .about p { margin: 0 0 10px; }
  .about .row { flex-wrap: wrap; align-items: center; }
  .ver { font-weight: normal; color: var(--muted); }
  .setting { display: flex; justify-content: space-between; align-items: center; gap: 16px; }
  .setting p { margin: 4px 0 0; }
  .setting .buttons { display: flex; gap: 6px; }
  .setting + .setting { margin-top: 2px; padding-top: 12px; border-top: 1px solid var(--line); }
  .segmented { display: flex; padding: 2px; gap: 2px; border: 1px solid var(--line-strong); border-radius: var(--radius); background: var(--field); }
  .segmented button { border: 0; padding: 3px 10px; background: transparent; color: var(--muted); }
  .segmented button:hover:not(:disabled) { background: var(--hover); color: var(--text); }
  .segmented button.on { background: var(--raised); color: var(--text); }
  .login { display: grid; gap: 12px; }
  .login p { margin: 0; }
  .login .where { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: var(--mono); font-size: 12px; }
  .fine { font-size: 12px; }
  .end { display: flex; justify-content: flex-end; gap: 8px; }
  .notices { max-height: 28vh; }
  .licenses { border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); overflow: hidden; }
  .licenses details { border-bottom: 1px solid var(--line); }
  .licenses details:last-child { border-bottom: 0; }
  .licenses summary { padding: 6px 10px; cursor: pointer; }
  .licenses summary:hover { background: var(--hover); }
  .licenses details[open] summary { background: var(--raised); }
  .licenses details .output { margin: 8px 10px 10px; max-height: 40vh; }
  .lic { float: right; }
</style>
