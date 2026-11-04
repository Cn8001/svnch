import { getContext, setContext } from "svelte";
import { SvelteSet } from "svelte/reactivity";
import { Dialogs } from "@wailsio/runtime";
import {
  svn,
  SetupService,
  ServerService,
  TrustService,
  errorText,
  failureCode,
  hasChanges,
  isVersioned,
  relative,
  type BlameLine,
  type Info,
  type LogEntry,
  type MenuItem,
  type Server,
  type ServerCert,
  type Setup,
  type StatusEntry,
} from "./svn";

export const failed = Symbol("failed");
export type Failed = typeof failed;

// Dialog is the one window open over the main view, with what it was opened for; each dialog component keeps its
// own form state, so this only carries where it starts.
export type Dialog =
  | { kind: "checkout"; url: string; path: string }
  | { kind: "move"; path: string }
  | { kind: "confirm"; title: string; text: string; ok: string; run: () => void }
  | { kind: "merge" }
  | { kind: "shelve" }
  | { kind: "unshelve" }
  | { kind: "log"; target: string; purpose: string }
  | { kind: "help"; topic: string }
  | { kind: "text"; title: string; text: string }
  | { kind: "about" }
  | { kind: "settings" }
  | { kind: "repos"; at: string }
  | { kind: "svn" }
  | { kind: "details" }
  | { kind: "branch" }
  | { kind: "switch" }
  | { kind: "changelist"; paths: string[] }
  | { kind: "conflicts"; first: string }
  | { kind: "props"; target: string; label: string; isDir: boolean }
  | { kind: "lock"; paths: string[] }
  | { kind: "relocate" }
  | { kind: "import"; path: string };

export type Pane = "diff" | "history" | "blame";
export type Result<T> = { ok: true; value: T } | { ok: false; error: string };

const LAST_WC = "svnch.lastWorkingCopy";
const RECENT = "svnch.recentWorkingCopies";

export function baseName(path: string): string {
  return path.replace(/[\\/]+$/, "").split(/[\\/]/).pop() || path;
}

export function parentOf(path: string): string {
  return path.replace(/[\\/]+$/, "").replace(/[\\/][^\\/]*$/, "");
}

function readRecent(): string[] {
  try {
    const list = JSON.parse(localStorage.getItem(RECENT) ?? "[]");
    return Array.isArray(list) ? list.filter((x) => typeof x === "string") : [];
  } catch {
    return [];
  }
}

// Session is the open working copy and everything the window shows about it. App makes one and hands it down
// through context, so components share it without a module-level singleton.
export class Session {
  wc = $state("");
  info = $state<Info | null>(null);
  entries = $state<StatusEntry[]>([]);
  readonly checked = new SvelteSet<string>();
  message = $state("");
  recent = $state<string[]>(readRecent());

  focused = $state("");
  diffTitle = $state("");
  diffText = $state("");
  diffLoading = $state(false);
  private diffSeq = 0;

  pane = $state<Pane>("diff");
  history = $state({ entries: [] as LogEntry[], loading: false, wc: "" });
  blame = $state({ target: "", lines: [] as BlameLine[], loading: false });

  busy = $state("");
  cancelBusy = $state<(() => void) | null>(null);
  notice = $state("");
  error = $state("");
  errorFull = $state("");
  lastFailure = ""; // the code of the last failed run, for callers that word their own message

  dialog = $state<Dialog | null>(null);
  ctxMenu = $state<{ x: number; y: number; items: MenuItem[] } | null>(null);
  commitBox = $state<HTMLTextAreaElement | undefined>(undefined);

  setup = $state<Setup | null>(null);
  checkingSvn = $state(false);
  servers = $state<Server[]>([]);
  sessionUser = $state("");

  login = $state({ open: false, url: "", username: "", password: "", error: "", busy: false, manual: false }); // manual: opened by the user, so the url is editable
  private loginDone: ((ok: boolean) => void) | null = null;
  cert = $state<{ open: boolean; url: string; info: ServerCert | null; error: string; busy: boolean }>({ open: false, url: "", info: null, error: "", busy: false });
  private certDone: ((ok: boolean) => void) | null = null;

  svnMissing = $derived(!!this.setup && !this.setup.binary);
  conflicts = $derived(this.entries.filter((e) => e.item === "conflicted" || e.treeConflicted || e.props === "conflicted"));
  changelists = $derived([...new Set(this.entries.map((e) => e.changelist).filter(Boolean))].sort());
  canCommit = $derived(!!this.wc && !this.busy && !!this.message.trim() && this.entries.some((e) => hasChanges(e) || this.checked.has(e.path)));
  wcServer = $derived(this.info ? this.servers.find((s) => this.info!.url.startsWith(s.url)) : undefined);

  // Every svn call goes through here so the status bar, the cancel button, error reporting and the login prompt
  // behave the same. make is called again after a login, so a command that hit a password-protected server carries on.
  run = async <T>(label: string, make: () => Promise<T> & { cancel?: () => void }, url = ""): Promise<T | Failed> => {
    let logins = 0;
    let trusts = 0;
    let cleanups = 0;
    for (;;) {
      let cancelled = false;
      const p = make();
      this.busy = label;
      this.error = "";
      this.notice = "";
      this.cancelBusy = () => {
        cancelled = true;
        p.cancel?.();
      };
      try {
        return await p;
      } catch (e) {
        this.busy = "";
        this.cancelBusy = null;
        const code = cancelled ? "CANCELLED" : failureCode(e);
        this.lastFailure = code;
        const where = url || this.info?.repositoryRoot || "";
        if (code === "AUTH" && logins < 3 && where && (await this.askLogin(where, logins++ ? "That did not work, try again." : ""))) continue;
        if (code === "CERT" && trusts++ < 2 && where && (await this.askTrust(where))) continue;
        // A lock left by an interrupted command (a stopped update, a closed window) only needs svn cleanup, which
        // finishes or rolls back what was cut off; do it once and try again rather than send the person to a terminal.
        if (code === "NEEDS_CLEANUP" && cleanups++ < 1 && this.wc && (await this.cleanupFor(label))) continue;
        if (code === "CERT" && !this.cert.info) {
          this.show({
            kind: "text",
            title: "Untrusted server certificate",
            text: `${errorText(e)}\n\nsvn does not trust this server's HTTPS certificate. Check it once in a terminal and accept it permanently:\n\n  svn info ${where || "<url>"}\n\nthen press (p) when svn asks.`,
          });
        }
        if (code === "SVN_MISSING") await this.checkSvn();
        this.error = cancelled ? "Cancelled." : errorText(e);
        this.errorFull = cancelled ? "" : e instanceof Error ? e.message : String(e);
        return failed;
      } finally {
        this.busy = "";
        this.cancelBusy = null;
      }
    }
  };

  private async cleanupFor(label: string): Promise<boolean> {
    this.busy = "Cleaning up an interrupted operation";
    try {
      await svn.Cleanup(this.wc);
      return true;
    } catch {
      return false;
    } finally {
      this.busy = label;
    }
  }

  // remote is run for components that want the error text back rather than a sentinel.
  remote = async <T>(label: string, make: () => Promise<T>, url: string): Promise<Result<T>> => {
    const r = await this.run(label, make, url);
    return r === failed ? { ok: false, error: this.error || "It did not work." } : { ok: true, value: r };
  };

  show = (d: Dialog) => {
    this.dialog = d;
  };

  close = () => {
    this.dialog = null;
  };

  ask = (title: string, text: string, ok: string, run: () => void) => this.show({ kind: "confirm", title, text, ok, run });

  // askTrust shows the certificate and resolves true once it is trusted; false sends run on to the text
  // explanation only when the certificate could not even be read (an svn:// or proxy setup, say).
  askTrust = async (url: string): Promise<boolean> => {
    this.cert = { open: false, url, info: null, error: "", busy: false };
    try {
      this.cert.info = await TrustService.Cert(url);
    } catch {
      return false;
    }
    this.cert.open = true;
    return new Promise((resolve) => (this.certDone = resolve));
  };

  finishTrust = (ok: boolean) => {
    this.cert.open = false;
    this.certDone?.(ok);
    this.certDone = null;
  };

  submitTrust = async () => {
    if (!this.cert.info) return;
    this.cert.busy = true;
    this.cert.error = "";
    try {
      await TrustService.Trust(this.cert.url, this.cert.info.fingerprint);
    } catch (e) {
      this.cert.busy = false;
      this.cert.error = errorText(e);
      return;
    }
    this.cert.busy = false;
    this.finishTrust(true);
  };

  askLogin = (url: string, message = "", manual = false): Promise<boolean> => {
    const known = this.servers.find((s) => url.startsWith(s.url));
    this.login = { open: true, url, username: known?.username || this.login.username, password: "", error: message, busy: false, manual };
    return new Promise((resolve) => (this.loginDone = resolve));
  };

  finishLogin = (ok: boolean) => {
    this.login.open = false;
    this.loginDone?.(ok);
    this.loginDone = null;
  };

  startLogin = (url = "") => {
    this.askLogin(url || this.info?.repositoryRoot || this.servers[0]?.url || "", "", true);
  };

  submitLogin = async () => {
    const l = this.login;
    l.url = l.url.trim().replace(/\/+$/, "");
    if (!l.username || !l.url) return;
    l.busy = true;
    l.error = "";
    try {
      await svn.Login(l.url, l.username, l.password);
    } catch (e) {
      l.busy = false;
      l.error = failureCode(e) === "AUTH" ? "Wrong username or password." : errorText(e);
      return;
    }
    l.busy = false;
    this.sessionUser = l.username;
    const known = this.servers.find((s) => l.url.startsWith(s.url));
    if (known && known.username !== l.username) {
      try {
        await ServerService.Save({ ...known, username: l.username });
        await this.loadServers();
      } catch {}
    }
    this.finishLogin(true);
  };

  loadServers = async () => {
    try {
      this.servers = (await ServerService.List()) ?? [];
    } catch {}
  };

  checkSvn = async () => {
    this.checkingSvn = true;
    try {
      this.setup = await SetupService.Status();
    } catch {}
    this.checkingSvn = false;
  };

  entry = (path: string): StatusEntry | undefined => this.entries.find((e) => e.path === path);

  saveRecent = (list: string[]) => {
    this.recent = list.slice(0, 12);
    try {
      localStorage.setItem(RECENT, JSON.stringify(this.recent));
    } catch {}
  };

  lastOpened(): string {
    try {
      return localStorage.getItem(LAST_WC) ?? "";
    } catch {
      return "";
    }
  }

  private remember(path: string) {
    try {
      localStorage.setItem(LAST_WC, path);
    } catch {}
    this.saveRecent([path, ...this.recent.filter((p) => p !== path)]);
  }

  openFolder = async () => {
    const dir = await Dialogs.OpenFile({ Title: "Open working copy", ButtonText: "Open", CanChooseDirectories: true, CanChooseFiles: false });
    if (typeof dir === "string" && dir) await this.openWC(dir);
  };

  openWC = async (dir: string) => {
    const i = await this.run(`Reading ${dir}`, () => svn.Info(dir));
    if (i === failed) {
      if (this.lastFailure === "NOT_WORKING_COPY") this.saveRecent(this.recent.filter((p) => p !== dir));
      return;
    }
    this.info = i;
    this.wc = i.workingCopyRoot || dir;
    this.remember(this.wc);
    this.checked.clear();
    this.clearDiff();
    await this.refresh();
  };

  reloadInfo = async () => {
    try {
      this.info = await svn.Info(this.wc);
    } catch {}
    if (this.pane === "history") this.loadHistory();
  };

  refresh = async () => {
    if (!this.wc) return;
    const r = await this.run("Reading status", () => svn.Status(this.wc));
    if (r === failed) return;
    this.entries = (r ?? []).sort((a, b) => a.path.localeCompare(b.path));
    for (const p of [...this.checked]) if (!this.entry(p)) this.checked.delete(p);
    if (this.focused && !this.entry(this.focused)) this.clearDiff();
    else if (this.focused) await this.showDiff(this.focused);
  };

  clearDiff = () => {
    this.focused = "";
    this.diffTitle = "";
    this.diffText = "";
  };

  showDiff = async (path: string) => {
    this.focused = path;
    const e = this.entry(path);
    this.diffTitle = relative(this.wc, path);
    this.diffText = "";
    if (e && !isVersioned(e)) {
      this.diffTitle += "  (unversioned, add it to track it)";
      return;
    }
    if (e?.item === "missing") {
      this.diffTitle += "  (missing on disk)";
      return;
    }
    await this.loadDiff(() => svn.Diff({ target: path, change: 0 }));
  };

  showRevisionDiff = async (target: string, rev: number) => {
    this.pane = "diff";
    this.focused = "";
    this.diffTitle = `r${rev}  ${relative(this.wc, target)}`;
    this.diffText = "";
    await this.loadDiff(() => svn.Diff({ target, change: rev }));
  };

  // loadDiff drops a reply that arrives after a newer diff was asked for, so a slow one never replaces a fast one.
  private async loadDiff(get: () => Promise<string>) {
    const seq = ++this.diffSeq;
    this.diffLoading = true;
    try {
      const text = await get();
      if (seq === this.diffSeq) this.diffText = text;
    } catch (err) {
      if (seq === this.diffSeq) this.error = errorText(err);
    } finally {
      if (seq === this.diffSeq) this.diffLoading = false;
    }
  }

  // History is read when its tab opens and again after anything that moves the working copy, so "you are here"
  // and the list stay true without reading the log on every refresh.
  loadHistory = async () => {
    if (!this.wc) return;
    this.history.loading = true;
    // The url, not the folder: svn log on a working copy stops at its own revision and would hide what is newer.
    const target = this.info?.url ?? this.wc;
    const r = await this.run("Reading history", () => svn.Log({ target, limit: 200, verbose: true }), this.info?.repositoryRoot);
    this.history.loading = false;
    if (r === failed) return;
    this.history.entries = r ?? [];
    this.history.wc = this.wc;
  };

  showPane = (p: Pane) => {
    this.pane = p;
    if (p === "history" && (this.history.wc !== this.wc || !this.history.entries.length)) this.loadHistory();
  };

  openBlame = async (path: string) => {
    this.pane = "blame";
    this.blame = { target: path, lines: [], loading: true };
    const r = await this.run(`Blaming ${baseName(path)}`, () => svn.Blame(path), this.info?.repositoryRoot);
    if (this.blame.target !== path) return;
    this.blame.loading = false;
    if (r !== failed) this.blame.lines = r ?? [];
  };

  // An action on a checked row applies to every checked row, the way file managers treat a selection.
  targetsFor = (path: string): string[] => (this.checked.has(path) && this.checked.size > 1 ? [...this.checked] : [path]);

  names = (paths: string[]): string => {
    const shown = paths.slice(0, 8).map((p) => "  " + relative(this.wc, p));
    if (paths.length > 8) shown.push(`  ...and ${paths.length - 8} more`);
    return shown.join("\n");
  };

  under = (paths: string[]): StatusEntry[] =>
    this.entries.filter((e) => e.item !== "ignored" && paths.some((p) => e.path === p || (e.path.startsWith(p) && /[\\/]/.test(e.path[p.length]))));

  // A tree row has no kind, so a path counts as a folder when it is the working copy, has no status of its own (the
  // folder rows), or has changes below it.
  isFolder = (path: string): boolean =>
    path === this.wc || !this.entry(path) || this.entries.some((e) => e.path.startsWith(path) && /[\\/]/.test(e.path[path.length]));

  urlOf = (path: string): string => {
    if (!this.info) return path;
    if (path === this.wc) return this.info.url;
    return this.info.url + "/" + relative(this.wc, path).split(/[\\/]/).map(encodeURIComponent).join("/");
  };

  // layout splits the working copy's url into the usual trunk / branches / tags parts. A repository that holds
  // several projects keeps them per project (^/Game/trunk), so the folder above trunk is found, not assumed to be ^/.
  layout = () => {
    const info = this.info;
    if (!info) return null;
    const root = info.repositoryRoot.replace(/\/+$/, "");
    const rel = (info.relativeUrl || "^/").replace(/^\^/, "");
    const m = rel.match(/^(.*?)\/(trunk|branches\/[^/]+|tags\/[^/]+)(\/.*)?$/);
    if (!m) return { base: root, line: "", sub: "", from: info.url };
    const base = root + m[1];
    return { base, line: decodeURIComponent(m[2]), sub: m[3] ?? "", from: `${base}/${m[2]}` };
  };

  // atRoot is a working copy of the whole repository in a repository without trunk and branches. svn would copy the
  // root into ^/branches, but each such branch holds every branch before it, so svnch does not offer it there.
  atRoot = $derived.by(() => {
    const i = this.info;
    if (!i || this.layout()?.line) return false;
    return i.url.replace(/\/+$/, "") === i.repositoryRoot.replace(/\/+$/, "");
  });
}

const key = Symbol("session");

export function provideSession(s: Session) {
  setContext(key, s);
}

export function useSession(): Session {
  return getContext<Session>(key);
}
