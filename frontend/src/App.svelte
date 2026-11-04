<script lang="ts">
  import { onMount } from "svelte";
  import { SvelteSet } from "svelte/reactivity";
  import { Application, Browser, Clipboard, Dialogs } from "@wailsio/runtime";
  import MenuBar from "./lib/MenuBar.svelte";
  import FileList from "./lib/FileList.svelte";
  import DiffView from "./lib/DiffView.svelte";
  import HistoryView from "./lib/HistoryView.svelte";
  import ConflictPanel from "./lib/ConflictPanel.svelte";
  import PropertiesPanel from "./lib/PropertiesPanel.svelte";
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
    TrustService,
    failureCode,
    type ExplorerStatus,
    type Launch,
    type ServerCert,
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
  const RECENT = "svnch.recentWorkingCopies";
  let recent = $state<string[]>(readRecent());

  function readRecent(): string[] {
    try {
      const list = JSON.parse(localStorage.getItem(RECENT) ?? "[]");
      return Array.isArray(list) ? list.filter((x) => typeof x === "string") : [];
    } catch {
      return [];
    }
  }

  function saveRecent(list: string[]) {
    recent = list.slice(0, 12);
    try {
      localStorage.setItem(RECENT, JSON.stringify(recent));
    } catch {}
  }

  function baseName(path: string): string {
    return path.replace(/[\\/]+$/, "").split(/[\\/]/).pop() || path;
  }

  function parentOf(path: string): string {
    return path.replace(/[\\/]+$/, "").replace(/[\\/][^\\/]*$/, "");
  }

  // projectItems is the switcher behind the working copy's name in the toolbar.
  function projectItems(): MenuItem[] {
    return [
      ...recent.map((p) => ({ label: baseName(p), hint: parentOf(p), action: () => openWC(p), disabled: p === wc || !!busy })),
      ...(recent.length ? [{ separator: true as const }] : []),
      { label: "Open working copy...", action: openFolder, disabled: !!busy },
      { label: "Checkout...", action: () => openCheckout(), disabled: !!busy },
      ...(recent.length > 1 ? [{ label: "Clear this list", action: () => saveRecent(wc ? [wc] : []) }] : []),
    ];
  }
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

  type ModalKind = "checkout" | "move" | "confirm" | "merge" | "shelve" | "unshelve" | "log" | "help" | "text" | "about" | "settings" | "licenses" | "repos" | "svn" | "details" | "branch" | "switch" | "changelist" | "conflicts" | "props";

  let setup = $state<Setup | null>(null);
  let checkingSvn = $state(false);
  let servers: Server[] = $state([]);
  let login = $state({ url: "", username: "", password: "", error: "", busy: false, manual: false }); // manual: opened by the user, so the url is editable
  let sessionUser = $state("");
  let cert = $state<{ open: boolean; url: string; info: ServerCert | null; error: string; busy: boolean }>({ open: false, url: "", info: null, error: "", busy: false });
  let certDone: ((ok: boolean) => void) | null = null;
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
  let log = $state({ target: "", entries: [] as LogEntry[], selected: 0, purpose: "" });
  let br = $state({ name: "", message: "", switchAfter: true });
  let sw = $state({ url: "", choices: [] as { label: string; url: string }[], loading: false });
  let cl = $state({ paths: [] as string[], name: "" });
  let commitBox: HTMLTextAreaElement | undefined = $state();
  let help = $state({ topic: "", text: "" });
  let textBox = $state({ title: "", text: "" });

  const failed = Symbol("failed");
  let lastFailure = ""; // the code of the last failed run, for callers that word their own message

  // Every svn call goes through here so the status bar, the cancel button, error reporting and the login prompt behave
  // the same. make is called again after a login, so a command that hit a password-protected server just carries on.
  async function run<T>(label: string, make: () => Promise<T> & { cancel?: () => void }, url = ""): Promise<T | typeof failed> {
    let logins = 0;
    let trusts = 0;
    for (;;) {
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
        lastFailure = code;
        const where = url || info?.repositoryRoot || "";
        if (code === "AUTH" && logins < 3 && where && (await askLogin(where, logins++ ? "That did not work, try again." : ""))) continue;
        if (code === "CERT" && trusts++ < 2 && where && (await askTrust(where))) continue;
        if (code === "CERT" && !cert.info) {
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

  // askTrust shows the certificate and resolves true once it is trusted; false sends run on to the text
  // explanation only when the certificate could not even be read (an svn:// or proxy setup, say).
  async function askTrust(url: string): Promise<boolean> {
    cert = { open: false, url, info: null, error: "", busy: false };
    try {
      cert.info = await TrustService.Cert(url);
    } catch {
      return false;
    }
    cert.open = true;
    return new Promise((resolve) => (certDone = resolve));
  }

  function finishTrust(ok: boolean) {
    cert.open = false;
    certDone?.(ok);
    certDone = null;
  }

  async function submitTrust() {
    if (!cert.info) return;
    cert.busy = true;
    cert.error = "";
    try {
      await TrustService.Trust(cert.url, cert.info.fingerprint);
    } catch (e) {
      cert.busy = false;
      cert.error = errorText(e);
      return;
    }
    cert.busy = false;
    finishTrust(true);
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
    saveRecent([path, ...recent.filter((p) => p !== path)]);
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
    if (i === failed) {
      if (lastFailure === "NOT_WORKING_COPY") saveRecent(recent.filter((p) => p !== dir));
      return;
    }
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
    if (pane === "history") loadHistory();
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

  let pane = $state<"diff" | "history">("diff");
  let history = $state({ entries: [] as LogEntry[], loading: false, wc: "" });

  // History is read when its tab opens and again after anything that moves the working copy, so "you are here"
  // and the list stay true without reading the log on every refresh.
  async function loadHistory() {
    if (!wc) return;
    history.loading = true;
    // The url, not the folder: svn log on a working copy stops at its own revision and would hide what is newer.
    const target = info?.url ?? wc;
    const r = await run("Reading history", () => svn.Log({ target, limit: 200, verbose: true }), info?.repositoryRoot);
    history.loading = false;
    if (r === failed) return;
    history.entries = r ?? [];
    history.wc = wc;
  }

  function showPane(p: "diff" | "history") {
    pane = p;
    if (p === "history" && (history.wc !== wc || !history.entries.length)) loadHistory();
  }

  function historyItems(e: LogEntry): MenuItem[] {
    const here = info?.revision === e.revision;
    return [
      { label: here ? "You are at this revision" : `Go to r${e.revision}`, action: () => updateToRevision(e, wc), disabled: here || !!busy },
      { label: "Show changes", action: () => { pane = "diff"; showRevisionDiff(info?.url ?? wc, e.revision); } },
      { label: "Undo this commit...", action: () => undoRevision(e, wc), disabled: !!busy },
      { separator: true },
      { label: "Copy revision number", action: () => Clipboard.SetText(String(e.revision)) },
      { label: "Copy message", action: () => Clipboard.SetText(e.message) },
    ];
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

  // layout splits the working copy's url into the usual trunk / branches / tags parts. A repository that holds
  // several projects keeps them per project (^/Game/trunk), so the folder above trunk is found, not assumed to be ^/.
  function layout() {
    if (!info) return null;
    const root = info.repositoryRoot.replace(/\/+$/, "");
    const rel = (info.relativeUrl || "^/").replace(/^\^/, "");
    const m = rel.match(/^(.*?)\/(trunk|branches\/[^/]+|tags\/[^/]+)(\/.*)?$/);
    if (!m) return { base: root, line: "", sub: "", from: info.url };
    const base = root + m[1];
    return { base, line: decodeURIComponent(m[2]), sub: m[3] ?? "", from: `${base}/${m[2]}` };
  }

  // atRoot is a working copy of the whole repository in a repository without trunk and branches. svn would copy the
  // root into ^/branches, but each such branch holds every branch before it, so svnch does not offer it there.
  let atRoot = $derived.by(() => {
    const i: Info | null = info;
    if (!i || layout()?.line) return false;
    return i.url.replace(/\/+$/, "") === i.repositoryRoot.replace(/\/+$/, "");
  });

  function urlOf(path: string): string {
    if (!info) return path;
    if (path === wc) return info.url;
    return info.url + "/" + relative(wc, path).split(/[\\/]/).map(encodeURIComponent).join("/");
  }

  function firstLine(e: LogEntry): string {
    return e.message.split("\n")[0] || "(no message)";
  }

  function isURL(target: string): boolean {
    return /^[a-z][a-z0-9+.-]*:\/\//i.test(target) || target.startsWith("^/");
  }

  function undoRevision(e: LogEntry, target = log.target) {
    const what = target === wc ? "your working copy" : relative(wc, target);
    ask(
      `Undo r${e.revision}`,
      `svnch applies the opposite of r${e.revision} to ${what}:\n\n  ${firstLine(e)}\n\nNothing changes on the server until you commit it. If later commits touched the same lines, those files show as conflicted.`,
      "Undo",
      async () => {
        const r = await run(`Undoing r${e.revision}`, () => svn.Merge({ source: urlOf(target), target, change: -e.revision, dryRun: false }));
        if (r === failed) return;
        if (!message.trim()) message = `Undo r${e.revision}: ${firstLine(e)}`;
        await refresh();
        notice = `r${e.revision} is undone in your working copy. Commit to share it.`;
      },
    );
  }

  function updateToRevision(e: LogEntry, target = log.target) {
    const what = target === wc ? "Your working copy" : relative(wc, target);
    ask(
      `Update to r${e.revision}`,
      `${what} will show its files as they were at r${e.revision}. Your local changes stay.\n\nNothing on the server changes, and Update brings it back to the latest. You cannot commit files that are behind, so update before committing.`,
      "Update",
      async () => {
        const r = await run(`Updating to r${e.revision}`, () => svn.UpdateTo(target, e.revision));
        if (r === failed) return;
        await refresh();
        await reloadInfo();
        notice = `At revision ${r}.`;
      },
    );
  }

  function openBranch() {
    if (!info) return;
    br = { name: "", message: "", switchAfter: true };
    modal = "branch";
  }

  function branchURL(name: string): string {
    const l = layout();
    return l ? `${l.base}/branches/${encodeURIComponent(name.trim())}` : "";
  }

  async function submitBranch() {
    const l = layout();
    const name = br.name.trim();
    if (!l || !name) return;
    const to = branchURL(name);
    const message = br.message.trim() || `Create branch ${name}`;
    const switchAfter = br.switchAfter;
    modal = null;
    const rev = await run(`Creating branch ${name}`, () => svn.Branch({ from: l.from, to, message }), l.base);
    if (rev === failed) return;
    if (switchAfter && (await run(`Switching to ${name}`, () => svn.Switch(wc, to + l.sub), l.base)) === failed) return;
    if (switchAfter) {
      await reloadInfo();
      await refresh();
    }
    notice = `Branch ${name} created in r${rev}${switchAfter ? ", and this working copy is on it now" : ""}.`;
  }

  // openSwitch lists what the server really has: trunk when there is one, otherwise the folder the working copy
  // came from (the whole repository when it has no layout), then each branch. Each listing gets CheckConnection's
  // ten seconds, because Commit and Switch wait while a command runs.
  async function openSwitch() {
    const l = layout();
    const i = info;
    if (!l || !i) return;
    const root = i.repositoryRoot.replace(/\/+$/, "");
    sw = { url: i.url, choices: [], loading: true };
    modal = "switch";
    const top = await run("Reading branches", () => svn.CheckConnection(l.base), l.base);
    const dirs = top === failed ? [] : (top ?? []).filter((e) => e.kind === "dir").map((e) => e.name);
    if (dirs.includes("trunk")) sw.choices.push({ label: "trunk", url: `${l.base}/trunk` });
    else sw.choices.push({ label: l.base === root ? "whole repository (^/)" : `^${l.base.slice(root.length)}`, url: l.base });
    if (dirs.includes("branches")) {
      const r = await run("Reading branches", () => svn.CheckConnection(`${l.base}/branches`), l.base);
      if (r !== failed) {
        for (const e of r ?? []) if (e.kind === "dir") sw.choices.push({ label: `branches/${e.name}`, url: `${l.base}/branches/${encodeURIComponent(e.name)}` });
      }
    }
    sw.loading = false;
  }

  async function submitSwitch(url: string) {
    const l = layout();
    if (!l || !url.trim()) return;
    const known = sw.choices.some((c) => c.url === url);
    const target = known ? url + l.sub : url.trim();
    modal = null;
    const rev = await run("Switching", () => svn.Switch(wc, target), l.base);
    if (rev === failed) return;
    await reloadInfo();
    await refresh();
    notice = `Switched to ${info?.relativeUrl || target}, at revision ${rev}.`;
  }

  let changelists = $derived([...new Set(entries.map((e) => e.changelist).filter(Boolean))].sort());

  function openChangelist(paths: string[]) {
    cl = { paths, name: "" };
    modal = "changelist";
  }

  async function moveToChangelist(name: string) {
    const target = name.trim();
    if (!target) return;
    const paths = cl.paths;
    modal = null;
    if ((await run(`Moving to ${target}`, () => svn.SetChangelist(target, paths))) === failed) return;
    await refresh();
  }

  async function leaveChangelist(paths: string[]) {
    if ((await run("Removing from changelist", () => svn.RemoveChangelist(paths))) === failed) return;
    await refresh();
  }

  function changelistItems(name: string, paths: string[]): MenuItem[] {
    return [
      {
        label: "Commit this changelist...",
        action: () => {
          checked.clear();
          paths.forEach((p) => checked.add(p));
          if (!message.trim()) message = `${name}: `;
          commitBox?.focus();
        },
      },
      { label: "Select only these", action: () => { checked.clear(); paths.forEach((p) => checked.add(p)); } },
      { separator: true },
      { label: "Rename...", action: () => openChangelist(paths) },
      { label: "Dissolve changelist", action: () => leaveChangelist(paths) },
    ];
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

  function under(paths: string[]): StatusEntry[] {
    return entries.filter((e) => e.item !== "ignored" && paths.some((p) => e.path === p || (e.path.startsWith(p) && /[\\/]/.test(e.path[p.length]))));
  }

  // discard shows what will happen before it does it: changed files go back, new ones are deleted for good. The Go
  // side works it out again from svn's status, so this list is only for the person deciding.
  function discard(paths: string[]) {
    const all = under(paths);
    const gone = all.filter((e) => e.item === "unversioned" || e.item === "added").map((e) => e.path);
    const back = all.filter((e) => e.item !== "unversioned" && e.item !== "added").map((e) => e.path);
    if (!gone.length && !back.length) return;
    const parts = [];
    if (back.length) parts.push(`Undo the local changes in:\n\n${names(back)}`);
    if (gone.length) parts.push(`Delete these new files from disk, for good:\n\n${names(gone)}`);
    ask("Discard changes", parts.join("\n\n"), "Discard", async () => {
      const r = await run("Discarding changes", () => svn.Discard(wc, paths));
      if (r !== failed) notice = `Discarded: ${r.reverted} reverted, ${r.removed} deleted.`;
      checked.clear();
      clearDiff();
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
      ...(e && (e.item === "conflicted" || e.treeConflicted || e.props === "conflicted")
        ? [{ label: "Resolve conflict...", action: () => openConflicts(path) }]
        : []),
      { label: "Discard changes..." + n, action: () => discard(paths), disabled: !under(paths).length },
      ...(unversioned.length ? [{ label: "Add to ignore list" + (unversioned.length > 1 ? ` (${unversioned.length})` : ""), action: () => ignore(unversioned) }] : []),
      { label: "Delete..." + n, action: () => remove(paths, false), disabled: unversioned.length > 0 },
      { label: "Remove from svn, keep file..." + n, action: () => remove(paths, true), disabled: unversioned.length > 0 },
      { label: "Rename / move...", action: () => openMove(path), disabled: many || !versioned },
      { separator: true },
      { label: "Move to changelist..." + n, action: () => openChangelist(sel.filter(isVersioned).map((x) => x.path)), disabled: !sel.some(isVersioned) },
      { label: "Remove from changelist", action: () => leaveChangelist(sel.filter((x) => x.changelist).map((x) => x.path)), disabled: !sel.some((x) => x.changelist) },
      { separator: true },
      { label: "Show log", action: () => openLog(path), disabled: many || !versioned || e?.item === "added" || e?.item === "ignored" },
      { label: "Properties...", action: () => openProps(path), disabled: many || !versioned },
      { label: "Copy path", action: () => Clipboard.SetText(path) },
    ];
  }

  let commitLabel = $derived(checked.size ? `Commit ${checked.size} selected` : "Commit all changes");
  let canCommit = $derived(!!wc && !busy && !!message.trim() && entries.some((e) => hasChanges(e) || checked.has(e.path)));

  let conflicts = $derived(entries.filter((e) => e.item === "conflicted" || e.treeConflicted || e.props === "conflicted"));
  let conflictFirst = $state("");
  let propsOf = $state({ target: "", label: "", isDir: true });

  // A tree row has no kind, so a path counts as a folder when it is the working copy, has no status of its own (the
  // folder rows), or has changes below it.
  function isFolder(path: string): boolean {
    return path === wc || !entry(path) || entries.some((e) => e.path.startsWith(path) && /[\\/]/.test(e.path[path.length]));
  }

  function openProps(path: string, dir = isFolder(path)) {
    propsOf = { target: path, label: path === wc ? baseName(wc) : relative(wc, path), isDir: dir };
    modal = "props";
  }

  async function pickProps() {
    const picked = await Dialogs.OpenFile({ Title: "Properties of", Directory: wc, CanChooseFiles: true, CanChooseDirectories: true, ButtonText: "Show properties" });
    if (typeof picked === "string" && picked) openProps(picked, isFolder(picked) && !/\.[^\\/.]+$/.test(picked));
  }

  async function ignore(paths: string[]) {
    if ((await run("Adding to the ignore list", () => svn.Ignore(paths))) === failed) return;
    notice = `Ignored ${paths.length === 1 ? baseName(paths[0]) : `${paths.length} items`}. The folder's svn:ignore changed; commit it to share.`;
    await refresh();
  }

  function openConflicts(path = "") {
    conflictFirst = path;
    modal = "conflicts";
  }

  async function resolveConflicts(accept: string, paths: string[]): Promise<boolean> {
    const r = await run("Resolving", () => svn.Resolve(accept, paths));
    await refresh();
    if (r === failed) return false;
    if (!conflicts.length) {
      modal = null;
      notice = "All conflicts are resolved. You can commit now.";
    }
    return true;
  }

  async function openLocal(path: string) {
    try {
      await ExplorerService.OpenFile(path);
    } catch (e) {
      error = errorText(e);
    }
  }

  // commit always updates first: svn refuses files that are behind, and committing on an old base is how someone
  // else's work gets missed. It stops when the update brings conflicts, which have to be settled before committing.
  async function commit() {
    if (!canCommit) return;
    const up = await run("Updating before commit", () => svn.Update([wc]));
    if (up === failed) return;
    await refresh();
    await reloadInfo();
    if (conflicts.length) {
      notice = `Updated to r${up}, and ${conflicts.length} ${conflicts.length === 1 ? "file conflicts" : "files conflict"}. Resolve them, then commit.`;
      openConflicts();
      return;
    }
    if (!canCommit) {
      notice = `Updated to r${up}. Nothing is left to commit.`;
      return;
    }
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
    notice = conflicts.length ? `At revision ${rev}, with ${conflicts.length} conflicting ${conflicts.length === 1 ? "file" : "files"}.` : `At revision ${rev}.`;
  }

  function openCheckout(url = "", path = "") {
    co = { url, path, revision: "", depth: Depth.DepthDefault };
    modal = "checkout";
  }

  let reposAt = $state(""); // where the browser opens, when it is opened for a known place

  async function openRepos(at = "") {
    await loadServers();
    reposAt = at;
    modal = "repos";
  }

  let wcServer = $derived(info ? servers.find((s) => info!.url.startsWith(s.url)) : undefined);

  function host(url: string): string {
    try {
      return new URL(url).host || url;
    } catch {
      return url;
    }
  }

  async function openDetails() {
    if (!wc) return;
    modal = "details";
    await reloadInfo();
    try {
      sessionUser = await svn.LoggedInAs();
    } catch {}
  }

  function wcItems(): MenuItem[] {
    return [
      { label: "Details...", action: openDetails },
      { separator: true },
      { label: "Properties...", action: () => openProps(wc, true) },
      { label: "Copy URL", action: () => info && Clipboard.SetText(info.url) },
      { label: "Copy repository root", action: () => info && Clipboard.SetText(info.repositoryRoot) },
      { label: "Copy working copy path", action: () => Clipboard.SetText(wc) },
      { separator: true },
      { label: "Browse on server...", action: () => info && openRepos(info.url), disabled: !!busy },
      { label: "Switch branch...", action: openSwitch, disabled: !!busy },
      { label: "New branch...", action: openBranch, disabled: !!busy },
      { label: "Show log", action: () => openLog(wc), disabled: !!busy },
      { label: "Update", action: update, disabled: !!busy },
      { label: "Log in...", action: () => startLogin(info?.repositoryRoot) },
    ];
  }

  async function listRepo(url: string) {
    const r = await run(`Listing ${url}`, () => svn.List(url), url);
    if (r !== failed) return r ?? [];
    return lastFailure === "TIMEOUT" ? "No answer within a minute." : error || "Could not list it.";
  }

  // The check goes through run, so a server that wants a password brings up the login with the form's username.
  async function checkServer(url: string, username: string) {
    const target = url.trim().replace(/\/+$/, "");
    if (username) login.username = username;
    const r = await run(`Connecting to ${target}`, () => svn.CheckConnection(target), target);
    if (r === failed) {
      const hint = target.startsWith("http://") ? " The server may only answer on https://." : " Check the address, or whether you need a VPN.";
      const text = lastFailure === "TIMEOUT" ? `No answer in 10 seconds.${hint}` : error || "Could not connect.";
      return { ok: false, text, username: "" };
    }
    const n = (r ?? []).length;
    return {
      ok: true,
      text: `Connected. ${n} ${n === 1 ? "entry" : "entries"} at this address.`,
      username: login.url.startsWith(target) ? login.username : "",
    };
  }

  // remote hands the repository browser the same run as everything else, so its server-side changes get the login
  // prompt and the status bar, and gives back the error text instead of a sentinel.
  async function remote<T>(label: string, make: () => Promise<T>, url: string): Promise<{ ok: true; value: T } | { ok: false; error: string }> {
    const r = await run(label, make, url);
    return r === failed ? { ok: false, error: error || "It did not work." } : { ok: true, value: r };
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
    if (r === failed) {
      // svn can fail at the very end (an external, a warning it counts as an error) after the working copy is
      // complete; open what is there, and leave the error in the status bar.
      const why = errorFull;
      try {
        await svn.Info(req.path);
      } catch {
        return;
      }
      await openWC(req.path);
      error = errorText(why);
      errorFull = why;
      return;
    }
    await openWC(req.path);
    notice = `Checked out revision ${r}.`;
  }

  async function openLog(target: string, purpose = "") {
    log = { target, entries: [], selected: 0, purpose };
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

  // An Explorer entry starts svnch with what to do and where. A folder that is not a working copy yet is where a
  // checkout goes, whatever the entry was.
  async function openStart(l: Launch) {
    const dir = l.isDir ? l.path : l.path.replace(/[\\/][^\\/]*$/, "");
    try {
      await svn.Info(dir);
    } catch (e) {
      if (failureCode(e) === "NOT_WORKING_COPY") {
        openCheckout("", dir);
        return;
      }
    }
    await openWC(dir);
    if (!wc) return;
    const inside = (p: string) => p === l.path || (p.startsWith(l.path) && /[\\/]/.test(p[l.path.length]));
    switch (l.action) {
      case "update": {
        const rev = await run("Updating", () => svn.Update([l.path]));
        if (rev === failed) return;
        await refresh();
        await reloadInfo();
        notice = `At revision ${rev}.`;
        break;
      }
      case "commit":
        checked.clear();
        entries.filter((e) => hasChanges(e) && inside(e.path)).forEach((e) => checked.add(e.path));
        commitBox?.focus();
        break;
      case "log":
        await openLog(l.path);
        break;
      case "diff":
        await showDiff(l.path);
        break;
    }
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
        { label: "Browse repositories...", action: () => openRepos(), hint: "Ctrl+B", disabled: !!busy || svnMissing },
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
        { label: "Resolve conflicts...", action: () => openConflicts(), disabled: !conflicts.length },
        { label: "Properties...", action: () => openProps(wc, true), disabled: !wc },
        { label: "Properties of a file or folder...", action: pickProps, disabled: !wc },
        { label: "Discard all changes...", action: () => discard([wc]), disabled: !wc || !!busy || !entries.some((e) => e.item !== "ignored") },
        { separator: true as const },
        { label: "Details...", action: openDetails, disabled: !wc },
        { label: "Show log", action: () => openLog(wc), disabled: !wc || !!busy },
        { label: "Undo a commit...", action: () => openLog(wc, "Pick the commit to undo, then press Undo this revision."), disabled: !wc || !!busy },
        { label: "Merge...", action: openMerge, disabled: !wc || !!busy },
        { separator: true as const },
        { label: "New branch...", action: openBranch, disabled: !wc || !!busy },
        { label: "Switch branch...", action: openSwitch, disabled: !wc || !!busy },
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
        { label: "Report a problem...", action: () => Browser.OpenURL(`${REPO_URL}/issues`) },
        { label: "svnch on GitHub", action: () => Browser.OpenURL(REPO_URL) },
      ],
    },
    { title: "Settings", action: openSettings },
    { title: "About", action: openAbout },
  ]);

  function onKey(e: KeyboardEvent) {
    if (modal || loginOpen || cert.open || ctxMenu) return;
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
    let start: Launch | null = null;
    try {
      start = await AppService.Launch();
    } catch {}
    if (start?.path) return openStart(start);
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
    <button onclick={() => openRepos()} disabled={!!busy || svnMissing}>Repos</button>
    <span class="sep"></span>
    <button onclick={update} disabled={!wc || !!busy}>Update</button>
    <button onclick={refresh} disabled={!wc || !!busy}>Refresh</button>
    <button onclick={() => openLog(wc)} disabled={!wc || !!busy}>Log</button>
    {#if info}
      <div class="where" role="button" tabindex="-1" title="Right-click for details" oncontextmenu={(e) => { e.preventDefault(); ctxMenu = { x: e.clientX, y: e.clientY, items: wcItems() }; }} ondblclick={openDetails}>
        <button
          class="project"
          title="{wc}, click to switch to another working copy"
          onclick={(e) => { const b = (e.currentTarget as HTMLElement).getBoundingClientRect(); ctxMenu = { x: b.left, y: b.bottom + 4, items: projectItems() }; }}
        >
          <span class="pname">{baseName(wc)}</span>
          <svg viewBox="0 0 16 16" width="11" height="11"><path d="M4 6l4 4 4-4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" /></svg>
        </button>
        <button class="branch" title="{info.url}, click to switch branch" onclick={openSwitch} disabled={!!busy}>{info.relativeUrl || info.url}</button>
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
        <button onclick={() => openRepos()} disabled={!!busy}>Browse repositories...</button>
      </div>
      {#if recent.length}
        <ul class="recent">
          {#each recent as p (p)}
            <li><button onclick={() => openWC(p)} disabled={!!busy} title={p}><b>{baseName(p)}</b><span class="muted">{parentOf(p)}</span></button></li>
          {/each}
        </ul>
      {/if}
    </div>
  {:else}
    <main>
      <section class="left">
        {#if conflicts.length}
          <div class="banner">
            <span><b>{conflicts.length}</b> {conflicts.length === 1 ? "file conflicts" : "files conflict"}. Commit waits until they are resolved.</span>
            <button class="small" onclick={() => openConflicts()}>Resolve...</button>
          </div>
        {/if}
        <div class="files">
          <FileList
            {wc}
            {entries}
            {checked}
            {focused}
            onfocus={showDiff}
            ontoggle={(p) => (checked.has(p) ? checked.delete(p) : checked.add(p))}
            onlythese={(ps) => {
              checked.clear();
              ps.forEach((p) => checked.add(p));
            }}
            oncontext={(p, e) => (ctxMenu = { x: e.clientX, y: e.clientY, items: contextItems(p) })}
            onlistcontext={(name, paths, e) => (ctxMenu = { x: e.clientX, y: e.clientY, items: changelistItems(name, paths) })}
          />
        </div>
        <form class="commit" onsubmit={(e) => { e.preventDefault(); commit(); }}>
          <textarea bind:value={message} bind:this={commitBox} placeholder="Commit message" rows="4"></textarea>
          <button class="primary" type="submit" disabled={!canCommit} title="Updates first, then commits">{commitLabel}</button>
        </form>
      </section>
      <section class="right">
        <div class="tabs" role="tablist">
          <button role="tab" aria-selected={pane === "diff"} class:on={pane === "diff"} onclick={() => showPane("diff")}>Diff</button>
          <button role="tab" aria-selected={pane === "history"} class:on={pane === "history"} onclick={() => showPane("history")}>History</button>
        </div>
        <div class="pane">
          {#if pane === "diff"}
            <DiffView title={diffTitle} text={diffText} loading={diffLoading} />
          {:else}
            <HistoryView
              entries={history.entries}
              current={info?.revision ?? 0}
              loading={history.loading}
              onmenu={(e, ev) => (ctxMenu = { x: ev.clientX, y: ev.clientY, items: historyItems(e) })}
              onopen={(e) => { pane = "diff"; showRevisionDiff(info?.url ?? wc, e.revision); }}
              onreload={loadHistory}
            />
          {/if}
        </div>
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
        <button type="button" onclick={() => openRepos()}>Browse...</button>
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
    {#if log.purpose}<p class="purpose">{log.purpose}</p>{/if}
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
      {#if wc && !isURL(log.target)}
        <button disabled={!log.entries[log.selected] || !!busy} onclick={() => updateToRevision(log.entries[log.selected])}>Update to this revision</button>
        <button disabled={!log.entries[log.selected] || !!busy} onclick={() => undoRevision(log.entries[log.selected])}>Undo this revision...</button>
        <span class="grow"></span>
      {/if}
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
          <p class="muted">An svnch menu when you right-click a folder or file: Update, Commit, Show log, Diff. On Windows 11 it is under "Show more options".</p>
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
      start={reposAt}
      onlogin={startLogin}
      exec={remote}
      known={info ? [info.repositoryRoot] : []}
      cancel={() => cancelBusy?.()}
    />
  </Modal>
{:else if modal === "branch" && info}
  <Modal title="New branch" onclose={() => (modal = null)}>
    {#if atRoot}
      <p>This working copy is the whole repository (<code>^/</code>), and the repository has no <code>trunk</code> and <code>branches</code> folders.</p>
      <p class="muted">
        A branch in svn is a copy of a folder. svn does let you copy the root into <code>^/branches</code>, but every
        such branch holds the whole repository, the branches made before it included, so they nest deeper each time.
        Branches need the usual layout: the files under <code>^/trunk</code>, branches under <code>^/branches</code>.
        Moving the files there is a single commit, but it changes the address for everyone, so agree on it with your
        team first. VisualSVN Server can also create new repositories with this layout.
      </p>
    {:else}
      <p class="muted">
        Copies {layout()?.line || info.relativeUrl} on the server as a new branch, in one commit. No files are transferred.
      </p>
      <!-- svelte-ignore a11y_autofocus -->
      <label><span>Name</span><input bind:value={br.name} placeholder="feature-login" autofocus /></label>
      {#if br.name.trim()}<p class="muted fine mono">{branchURL(br.name)}</p>{/if}
      <label><span>Message</span><input bind:value={br.message} placeholder={`Create branch ${br.name.trim() || "..."}`} /></label>
      <label class="check"><input type="checkbox" bind:checked={br.switchAfter} /> Switch this working copy to it</label>
      {#if !layout()?.line}<p class="muted fine">This repository has no trunk or branches folder; the branch goes under ^/branches.</p>{/if}
    {/if}
    {#snippet footer()}
      <button onclick={() => (modal = null)}>{atRoot ? "Close" : "Cancel"}</button>
      {#if !atRoot}
        <button class="primary" disabled={!br.name.trim() || /[\\/]/.test(br.name) || !!busy} onclick={submitBranch}>Create branch</button>
      {/if}
    {/snippet}
  </Modal>
{:else if modal === "switch" && info}
  <Modal title="Switch branch" onclose={() => (modal = null)}>
    <p class="muted">Points this working copy at another branch. Your local changes come along; files that differ are updated.</p>
    {#if !layout()?.line && !sw.loading && sw.choices.length <= 1}
      <p class="muted fine">
        {atRoot ? "This working copy is the whole repository, which has no trunk and branches folders" : "This repository has no trunk and branches folders"},
        so there is nothing else to list. A url still works.
      </p>
    {/if}
    {#if sw.choices.length || sw.loading}<ul class="choices">
      {#each sw.choices as c (c.url)}
        <li>
          <button class:sel={info.url.startsWith(c.url + "/") || info.url === c.url} onclick={() => submitSwitch(c.url)} disabled={!!busy}>
            <span>{c.label}</span>
            {#if info.url.startsWith(c.url + "/") || info.url === c.url}<span class="muted">current</span>{/if}
          </button>
        </li>
      {/each}
      {#if sw.loading}<li class="muted loadrow">Reading branches... <button class="small" onclick={() => cancelBusy?.()}>Stop</button></li>{/if}
    </ul>{/if}
    <label><span>Or any url</span><input bind:value={sw.url} class="mono" /></label>
    {#snippet footer()}
      <button onclick={() => (modal = null)}>Cancel</button>
      <button class="primary" disabled={!sw.url.trim() || sw.url === info?.url || !!busy} onclick={() => submitSwitch(sw.url)}>Switch to url</button>
    {/snippet}
  </Modal>
{:else if modal === "changelist"}
  <Modal title="Move to changelist" onclose={() => (modal = null)}>
    <p class="muted">{cl.paths.length} {cl.paths.length === 1 ? "file" : "files"}. A changelist groups changes so you can commit them on their own; svn forgets it once they are committed.</p>
    {#if changelists.length}
      <div class="row wrap">
        {#each changelists as name}<button onclick={() => moveToChangelist(name)}>{name}</button>{/each}
      </div>
    {/if}
    <form class="row" onsubmit={(e) => { e.preventDefault(); moveToChangelist(cl.name); }}>
      <!-- svelte-ignore a11y_autofocus -->
      <input bind:value={cl.name} placeholder="New changelist name" autofocus />
      <button class="primary" type="submit" disabled={!cl.name.trim() || cl.name.trim().startsWith("-")}>Move</button>
    </form>
  </Modal>
{:else if modal === "props"}
  <Modal title="Properties: {propsOf.label}" wide onclose={() => (modal = null)}>
    <PropertiesPanel target={propsOf.target} label={propsOf.label} isDir={propsOf.isDir} remote={false} exec={remote} onchanged={refresh} />
  </Modal>
{:else if modal === "conflicts"}
  <Modal title="Resolve conflicts" wide onclose={() => (modal = null)}>
    {#if conflicts.length}
      <ConflictPanel {wc} {conflicts} first={conflictFirst} resolve={resolveConflicts} openFile={openLocal} />
    {:else}
      <p>No conflicts left.</p>
    {/if}
  </Modal>
{:else if modal === "details" && info}
  <Modal title="Working copy" onclose={() => (modal = null)}>
    <dl class="kv">
      <dt>Server</dt>
      <dd>{host(info.repositoryRoot)}{#if wcServer}<span class="muted"> &middot; saved as {wcServer.name}</span>{/if}</dd>
      <dt>Repository</dt>
      <dd class="mono">{info.repositoryRoot}</dd>
      <dt>Branch path</dt>
      <dd class="mono">{info.relativeUrl || "^/"}</dd>
      <dt>URL</dt>
      <dd class="mono">{info.url}</dd>
      <dt>Working copy</dt>
      <dd class="mono">{info.workingCopyRoot || wc}</dd>
      <dt>Revision</dt>
      <dd>r{info.revision}</dd>
      <dt>Last change</dt>
      <dd>{[`r${info.lastCommit.revision}`, info.lastCommit.author && `by ${info.lastCommit.author}`, info.lastCommit.date && new Date(info.lastCommit.date).toLocaleString()].filter(Boolean).join(" ")}</dd>
      <dt>Depth</dt>
      <dd>{info.depth || "infinity"}</dd>
      <dt>Logged in as</dt>
      <dd>{sessionUser || (wcServer?.username ? `${wcServer.username} (saved, svn keeps the password)` : "svn's own saved login, if any")}</dd>
      <dt>UUID</dt>
      <dd class="mono muted">{info.repositoryUuid}</dd>
    </dl>
    {#snippet footer()}
      <button onclick={() => Clipboard.SetText(info?.url ?? "")}>Copy URL</button>
      <button onclick={() => info && openRepos(info.url)}>Browse on server...</button>
      <button class="primary" onclick={() => (modal = null)}>Close</button>
    {/snippet}
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

{#if cert.open && cert.info}
  <Modal title="Untrusted certificate" onclose={() => finishTrust(false)}>
    <p class="certnote">
      <b>{cert.info.realm.replace("https://", "")}</b> sent a certificate this computer does not trust, because
      {cert.info.problems?.join(" and ")}.
    </p>
    <p class="muted fine">
      That is normal for a server inside a company network with its own certificate. Trust it if the fingerprint
      matches the one your server admin sees (VisualSVN Server Manager shows it under the server's certificate).
    </p>
    <dl class="kv">
      <dt>Issued to</dt>
      <dd>{cert.info.subject}</dd>
      <dt>Issued by</dt>
      <dd>{cert.info.issuer}</dd>
      {#if cert.info.names?.length}
        <dt>Names</dt>
        <dd>{cert.info.names.join(", ")}</dd>
      {/if}
      <dt>Valid</dt>
      <dd>{new Date(cert.info.validFrom).toLocaleDateString()} to {new Date(cert.info.validUntil).toLocaleDateString()}</dd>
      <dt>Fingerprint</dt>
      <dd class="mono">{cert.info.fingerprint}</dd>
    </dl>
    {#if cert.error}<p class="error">{cert.error}</p>{/if}
    <p class="muted fine">svnch saves it where svn keeps the certificates you accept, so TortoiseSVN trusts it too.</p>
    {#snippet footer()}
      <button onclick={() => finishTrust(false)}>Cancel</button>
      <button class="primary" disabled={cert.busy} onclick={submitTrust}>{cert.busy ? "Saving..." : "Trust this certificate"}</button>
    {/snippet}
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
  .app { display: grid; grid-template-rows: auto auto 1fr auto; grid-template-columns: minmax(0, 1fr); height: 100%; }

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
  .project { display: flex; align-items: center; gap: 6px; padding: 3px 8px; font-weight: 600; max-width: 32ch; }
  .project .pname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .recent { list-style: none; margin: 8px 0 0; padding: 0; display: grid; gap: 4px; width: min(520px, 90vw); text-align: left; }
  .recent button { display: flex; align-items: baseline; gap: 10px; width: 100%; border-color: transparent; background: transparent; padding: 6px 10px; }
  .recent button:hover:not(:disabled) { background: var(--hover); border-color: var(--line); }
  .recent span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: var(--mono); font-size: 12px; }
  .rev { padding: 1px 8px; border-radius: 10px; background: var(--blue-soft); color: var(--blue); font-family: var(--mono); font-size: 12px; font-weight: 700; }

  .welcome { display: grid; place-content: center; justify-items: center; gap: 14px; text-align: center; }
  .welcome img { width: 120px; height: 120px; }
  .welcome p { margin: 0; }
  .actions { display: flex; gap: 10px; }

  main { display: grid; grid-template-columns: minmax(300px, 36%) 1fr; min-height: 0; }
  .left { display: flex; flex-direction: column; min-height: 0; border-right: 1px solid var(--line); background: var(--surface); }
  .files { flex: 1; min-height: 0; }
  .banner { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 6px 10px; background: var(--del-bg); color: var(--del-ink); border-bottom: 1px solid var(--line); font-size: 12px; }
  .banner button { color: var(--text); }
  .commit { display: grid; gap: 8px; padding: 10px; border-top: 1px solid var(--line); }
  .commit button { padding: 6px 12px; }
  .right { display: grid; grid-template-rows: auto 1fr; min-width: 0; min-height: 0; background: var(--bg); }
  .pane { display: grid; min-height: 0; overflow: hidden; }
  .tabs { display: flex; gap: 2px; padding: 4px 8px 0; border-bottom: 1px solid var(--line); background: var(--surface); }
  .tabs button { border: 0; border-bottom: 2px solid transparent; border-radius: 0; background: transparent; color: var(--muted); padding: 5px 12px; font-weight: 600; }
  .tabs button:hover:not(:disabled) { background: transparent; color: var(--text); }
  .tabs button.on { color: var(--text); border-bottom-color: var(--accent); }

  /* Above the modal backdrop, so Stop is reachable while a window is open. */
  .status {
    position: relative;
    z-index: 45;
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

  .where[role="button"] { cursor: default; padding: 2px 4px 2px 8px; border-radius: var(--radius); }
  .where[role="button"]:hover { background: var(--hover); }
  .certnote { margin: 0; }
  .purpose { margin: 0; padding: 8px 10px; border-radius: var(--radius); background: var(--accent-soft); }
  .wrap { flex-wrap: wrap; align-items: center; }
  .mono { font-family: var(--mono); font-size: 12px; }
  .choices { margin: 0; padding: 4px; list-style: none; border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); max-height: 40vh; overflow: auto; }
  .choices button { display: flex; justify-content: space-between; width: 100%; border: 0; border-radius: 4px; background: transparent; text-align: left; padding: 5px 8px; font-family: var(--mono); font-size: 12px; }
  .choices button:hover:not(:disabled) { background: var(--hover); }
  .choices button.sel { background: var(--accent-soft); box-shadow: inset 2px 0 0 var(--accent); }
  .choices li.muted { padding: 5px 8px; }
  .choices .loadrow { display: flex; align-items: center; justify-content: space-between; }
  .choices .loadrow button { width: auto; font-family: inherit; border: 1px solid var(--line-strong); padding: 0 8px; }
  .where .branch { border: 0; background: transparent; padding: 1px 6px; color: var(--muted); font-family: var(--mono); font-size: 12px; overflow: hidden; text-overflow: ellipsis; }
  .where .branch:hover:not(:disabled) { background: var(--hover); color: var(--text); }
  .kv { display: grid; grid-template-columns: max-content 1fr; gap: 6px 16px; margin: 0; }
  .kv dt { color: var(--muted); }
  .kv dd { margin: 0; min-width: 0; overflow-wrap: anywhere; user-select: text; }
  .kv .mono { font-family: var(--mono); font-size: 12px; }
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
