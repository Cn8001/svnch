import { getContext, setContext } from "svelte";
import { Dialogs } from "@wailsio/runtime";
import { svn, AppService, ExplorerService, SetupService, failureCode, errorText, hasChanges, isVersioned, join, relative, type Launch, type LogEntry } from "./svn";
import { failed, baseName, type Session } from "./session.svelte";

function firstLine(e: LogEntry): string {
  return e.message.split("\n")[0] || "(no message)";
}

// Actions are the things a person asks svnch to do, each one svn command or a short run of them, with what to say
// before and after. They work on the Session they are given; the views only call them.
export class Actions {
  constructor(private s: Session) {}

  // commit always updates first: svn refuses files that are behind, and committing on an old base is how someone
  // else's work gets missed. It stops when the update brings conflicts, which have to be settled before committing.
  commit = async () => {
    const s = this.s;
    if (!s.canCommit) return;
    const up = await s.run("Updating before commit", () => svn.Update([s.wc]), "", await this.incoming(s.wc));
    if (up === failed) return;
    await s.refresh();
    await s.reloadInfo();
    if (s.conflicts.length) {
      const n = s.conflicts.length;
      s.notice = `Updated to r${up}, and ${n} ${n === 1 ? "file conflicts" : "files conflict"}. Resolve them, then commit.`;
      s.show({ kind: "conflicts", first: "" });
      return;
    }
    if (!s.canCommit) {
      s.notice = `Updated to r${up}. Nothing is left to commit.`;
      return;
    }
    const sel = [...s.checked];
    const toAdd = sel.filter((p) => {
      const e = s.entry(p);
      return e && !isVersioned(e);
    });
    if (toAdd.length && (await s.run("Adding new files", () => svn.Add(toAdd))) === failed) return;
    const total = sel.length || s.entries.filter(hasChanges).length;
    const rev = await s.run("Committing", () => svn.Commit({ paths: sel.length ? sel : [s.wc], message: s.message }), "", total);
    if (rev === failed) return;
    s.message = "";
    s.checked.clear();
    await s.refresh();
    await s.reloadInfo();
    s.notice = rev ? `Committed revision ${rev}.` : "Nothing to commit.";
  };

  update = async (paths?: string[]) => {
    const s = this.s;
    if (!s.wc || s.busy) return;
    const target = paths ?? [s.wc];
    const rev = await s.run("Updating", () => svn.Update(target), "", await this.incoming(target[0]));
    if (rev === failed) return;
    await s.refresh();
    await s.reloadInfo();
    const n = s.conflicts.length;
    s.notice = n ? `At revision ${rev}, with ${n} conflicting ${n === 1 ? "file" : "files"}.` : `At revision ${rev}.`;
  };

  // incoming asks the server how many items an update would bring, for the progress bar's total. It is only a total:
  // when the server cannot be asked, the update itself says why, so the error here is dropped.
  private incoming = async (path: string): Promise<number> => {
    try {
      return await svn.Incoming(path);
    } catch {
      return 0;
    }
  };

  add = async (paths: string[]) => {
    if ((await this.s.run("Adding", () => svn.Add(paths))) !== failed) this.s.notice = `Added ${paths.length} item(s).`;
    await this.s.refresh();
  };

  // discard shows what will happen before it does it: changed files go back, new ones are deleted for good. The Go
  // side works it out again from svn's status, so this list is only for the person deciding.
  discard = (paths: string[]) => {
    const s = this.s;
    const all = s.under(paths);
    const gone = all.filter((e) => e.item === "unversioned" || e.item === "added").map((e) => e.path);
    const back = all.filter((e) => e.item !== "unversioned" && e.item !== "added").map((e) => e.path);
    if (!gone.length && !back.length) return;
    const parts = [];
    if (back.length) parts.push(`Undo the local changes in:\n\n${s.names(back)}`);
    if (gone.length) parts.push(`Delete these new files from disk, for good:\n\n${s.names(gone)}`);
    s.ask("Discard changes", parts.join("\n\n"), "Discard", async () => {
      const r = await s.run("Discarding changes", () => svn.Discard(s.wc, paths));
      if (r !== failed) s.notice = `Discarded: ${r.reverted} reverted, ${r.removed} deleted.`;
      s.checked.clear();
      s.clearDiff();
      await s.refresh();
    });
  };

  remove = (paths: string[], keepLocal: boolean) => {
    const s = this.s;
    const text = keepLocal
      ? `Stop versioning these, but keep the files on disk:\n\n${s.names(paths)}`
      : `Delete these from disk and schedule them for removal:\n\n${s.names(paths)}`;
    s.ask(keepLocal ? "Remove from svn" : "Delete", text, keepLocal ? "Remove" : "Delete", async () => {
      if ((await s.run("Deleting", () => svn.Delete(paths, keepLocal))) !== failed) s.notice = `Scheduled ${paths.length} item(s) for deletion.`;
      await s.refresh();
    });
  };

  ignore = async (paths: string[]) => {
    const s = this.s;
    if ((await s.run("Adding to the ignore list", () => svn.Ignore(paths))) === failed) return;
    s.notice = `Ignored ${paths.length === 1 ? baseName(paths[0]) : `${paths.length} items`}. The folder's svn:ignore changed; commit it to share.`;
    await s.refresh();
  };

  unversionIgnore = (paths: string[]) => {
    const s = this.s;
    s.ask("Unversion and add to ignore list", `Stop versioning these and add them to their folder's ignore list. The files stay on disk:\n\n${s.names(paths)}`, "Unversion", async () => {
      if ((await s.run("Removing from svn", () => svn.Delete(paths, true))) === failed) return;
      await this.ignore(paths);
    });
  };

  leaveChangelist = async (paths: string[]) => {
    if ((await this.s.run("Removing from changelist", () => svn.RemoveChangelist(paths))) === failed) return;
    await this.s.refresh();
  };

  selectOnly = (paths: string[]) => {
    this.s.checked.clear();
    paths.forEach((p) => this.s.checked.add(p));
  };

  cleanup = async () => {
    if ((await this.s.run("Cleaning up", () => svn.Cleanup(this.s.wc))) !== failed) this.s.notice = "Cleaned up.";
    await this.s.refresh();
  };

  upgrade = async (dir = this.s.wc): Promise<boolean> => {
    if ((await this.s.run("Upgrading the working copy", () => svn.Upgrade(dir))) === failed) return false;
    this.s.notice = "The working copy now uses this svn's format.";
    return true;
  };

  unlock = async (paths: string[]) => {
    const s = this.s;
    if ((await s.run("Releasing the lock", () => svn.Unlock(paths, false), s.info?.repositoryRoot)) !== failed) s.notice = "Lock released.";
    await s.refresh();
  };

  exportCopy = async (path = this.s.wc) => {
    const s = this.s;
    const parent = await Dialogs.OpenFile({ Title: `Export ${baseName(path)} into`, CanChooseDirectories: true, CanChooseFiles: false, CanCreateDirectories: true, ButtonText: "Export here" });
    if (typeof parent !== "string" || !parent) return;
    const dest = join(parent, baseName(path));
    if ((await s.run(`Exporting ${baseName(path)}`, () => svn.ExportLocal(path, dest))) !== failed) s.notice = `Exported to ${dest}, without svn's files.`;
  };

  createRepo = (path: string) => {
    const s = this.s;
    s.ask(
      "Create repository",
      `Make an empty repository in\n\n  ${path}\n\nIt is reached as file:// on this computer only. For a team, a server such as VisualSVN Server is its usual home.`,
      "Create",
      async () => {
        const url = await s.run("Creating the repository", () => SetupService.CreateRepository(path));
        if (url === failed) return;
        s.notice = `Repository created: ${url}`;
        s.show({ kind: "checkout", url, path: "" });
      },
    );
  };

  pickFolder = async (title: string, button: string, create = false): Promise<string> => {
    const d = await Dialogs.OpenFile({ Title: title, CanChooseDirectories: true, CanChooseFiles: false, CanCreateDirectories: create, ButtonText: button });
    return typeof d === "string" ? d : "";
  };

  createPatch = async (paths: string[]) => {
    const s = this.s;
    const dest = await Dialogs.SaveFile({ Title: "Save patch", Filename: `${baseName(s.wc)}.patch`, ButtonText: "Save", CanCreateDirectories: true });
    if (!dest) return;
    if ((await s.run("Creating the patch", () => svn.CreatePatch(paths, dest))) !== failed) s.notice = `Patch saved to ${dest}`;
  };

  applyPatch = async () => {
    const s = this.s;
    const file = await Dialogs.OpenFile({ Title: "Apply patch", CanChooseFiles: true, CanChooseDirectories: false, Filters: [{ DisplayName: "Patches", Pattern: "*.patch;*.diff" }], ButtonText: "Apply" });
    if (typeof file !== "string" || !file) return;
    const out = await s.run("Applying the patch", () => svn.ApplyPatch(s.wc, file));
    if (out === failed) return;
    await s.refresh();
    s.show({ kind: "text", title: "Patch applied", text: out.trim() || "The patch changed nothing." });
  };

  diffPrevious = async (path: string) => {
    const i = await this.s.run("Reading info", () => svn.Info(path));
    if (i === failed) return;
    await this.s.showRevisionDiff(path, i.lastCommit.revision);
  };

  undoRevision = (e: LogEntry, target: string) => {
    const s = this.s;
    const what = target === s.wc ? "your working copy" : relative(s.wc, target);
    s.ask(
      `Undo r${e.revision}`,
      `svnch applies the opposite of r${e.revision} to ${what}:\n\n  ${firstLine(e)}\n\nNothing changes on the server until you commit it. If later commits touched the same lines, those files show as conflicted.`,
      "Undo",
      async () => {
        // svn merges only into a working copy at one revision, which it is not right after a commit; update first.
        if ((await s.run("Updating before undo", () => svn.Update([s.wc]), "", await this.incoming(s.wc))) === failed) return;
        const r = await s.run(`Undoing r${e.revision}`, () => svn.Merge({ source: s.urlOf(target), target, change: -e.revision, dryRun: false }));
        if (r === failed) return;
        if (!s.message.trim()) s.message = `Undo r${e.revision}: ${firstLine(e)}`;
        await s.refresh();
        s.notice = `r${e.revision} is undone in your working copy. Commit to share it.`;
      },
    );
  };

  updateToRevision = (e: LogEntry, target: string) => {
    const s = this.s;
    const what = target === s.wc ? "Your working copy" : relative(s.wc, target);
    s.ask(
      `Update to r${e.revision}`,
      `${what} will show its files as they were at r${e.revision}. Your local changes stay.\n\nNothing on the server changes, and Update brings it back to the latest. You cannot commit files that are behind, so update before committing.`,
      "Update",
      async () => {
        const r = await s.run(`Updating to r${e.revision}`, () => svn.UpdateTo(target, e.revision));
        if (r === failed) return;
        await s.refresh();
        await s.reloadInfo();
        s.notice = `At revision ${r}.`;
      },
    );
  };

  resolveConflicts = async (accept: string, paths: string[]): Promise<boolean> => {
    const s = this.s;
    const r = await s.run("Resolving", () => svn.Resolve(accept, paths));
    await s.refresh();
    if (r === failed) return false;
    if (!s.conflicts.length) {
      s.close();
      s.notice = "All conflicts are resolved. You can commit now.";
    }
    return true;
  };

  openLocal = async (path: string) => {
    try {
      await ExplorerService.OpenFile(path);
    } catch (e) {
      this.s.error = errorText(e);
    }
  };

  openProps = (path: string, isDir = this.s.isFolder(path)) => {
    const s = this.s;
    s.show({ kind: "props", target: path, label: path === s.wc ? baseName(s.wc) : relative(s.wc, path), isDir });
  };

  pickProps = async () => {
    const s = this.s;
    const picked = await Dialogs.OpenFile({ Title: "Properties of", Directory: s.wc, CanChooseFiles: true, CanChooseDirectories: true, ButtonText: "Show properties" });
    if (typeof picked === "string" && picked) this.openProps(picked, s.isFolder(picked) && !/\.[^\\/.]+$/.test(picked));
  };

  openRepos = async (at = "") => {
    await this.s.loadServers();
    this.s.show({ kind: "repos", at });
  };

  openDetails = async () => {
    const s = this.s;
    if (!s.wc) return;
    s.show({ kind: "details" });
    await s.reloadInfo();
    try {
      s.sessionUser = await svn.LoggedInAs();
    } catch {}
  };

  openLog = (target: string, purpose = "") => this.s.show({ kind: "log", target, purpose });

  openCheckout = (url = "", path = "") => this.s.show({ kind: "checkout", url, path });

  showError = () => {
    const s = this.s;
    if (s.errorFull) s.show({ kind: "text", title: "Error", text: s.errorFull });
  };

  // start runs what svnch was launched for: a folder or file from the command line, or an Explorer menu entry.
  start = async () => {
    const s = this.s;
    s.loadServers();
    await s.checkSvn();
    if (s.svnMissing) return;
    let l: Launch | null = null;
    try {
      l = await AppService.Launch();
    } catch {}
    if (l?.path) return this.openStart(l);
    const last = s.lastOpened();
    if (last) s.openWC(last);
  };

  // An Explorer entry starts svnch with what to do and where. A folder that is not a working copy yet is where a
  // checkout goes, whatever the entry was.
  private openStart = async (l: Launch) => {
    const s = this.s;
    const dir = l.isDir ? l.path : l.path.replace(/[\\/][^\\/]*$/, "");
    // These do not need a working copy, and some are for a folder that is not one yet.
    switch (l.action) {
      case "checkout":
        return this.openCheckout("", dir);
      case "import":
        return s.show({ kind: "import", path: dir });
      case "createrepo":
        return this.createRepo(dir);
      case "settings":
        return s.show({ kind: "settings" });
      case "help":
        return s.show({ kind: "help", topic: "" });
      case "about":
        return s.show({ kind: "about" });
      case "upgrade":
        if (!(await this.upgrade(dir))) return;
        break;
    }
    try {
      await svn.Info(dir);
    } catch (e) {
      if (failureCode(e) === "NOT_WORKING_COPY") {
        if (l.action !== "open") s.notice = `${dir} is not a working copy, so there is nothing to ${l.action} here. Check one out into it?`;
        this.openCheckout("", dir);
        return;
      }
    }
    await s.openWC(dir);
    if (!s.wc) return;
    const inside = (p: string) => p === l.path || (p.startsWith(l.path) && /[\\/]/.test(p[l.path.length]));
    const mine = s.entries.filter((e) => inside(e.path));
    switch (l.action) {
      case "update":
        return this.update([l.path]);
      case "commit":
        this.selectOnly(mine.filter(hasChanges).map((e) => e.path));
        s.commitBox?.focus();
        return;
      case "log":
        return this.openLog(l.path);
      case "diff":
        return s.showDiff(l.path);
      case "diffprev":
        return this.diffPrevious(l.path);
      case "blame":
        return s.openBlame(l.path);
      case "repos":
        return this.openRepos(s.urlOf(l.path));
      case "updaterev":
        return s.showPane("history");
      case "revert":
        return this.discard([l.path]);
      case "cleanup":
        return this.cleanup();
      case "resolve":
        return s.show({ kind: "conflicts", first: s.conflicts.some((c) => c.path === l.path) ? l.path : "" });
      case "lock":
        return s.show({ kind: "lock", paths: [l.path] });
      case "unlock":
        return this.unlock([l.path]);
      case "branch":
        return s.show({ kind: "branch" });
      case "switch":
        return s.show({ kind: "switch" });
      case "merge":
        return s.show({ kind: "merge" });
      case "export":
        return this.exportCopy(l.path);
      case "relocate":
        return s.show({ kind: "relocate" });
      case "add": {
        const fresh = l.isDir ? mine.filter((e) => e.item === "unversioned").map((e) => e.path) : [l.path];
        if (fresh.length) return this.add(fresh);
        s.notice = "Nothing new to add here.";
        return;
      }
      case "ignore":
        return this.ignore([l.path]);
      case "unversion":
        return this.unversionIgnore([l.path]);
      case "rename":
        return s.show({ kind: "move", path: l.path });
      case "delete":
        return this.remove([l.path], false);
      case "shelve":
        return s.show({ kind: "shelve" });
      case "unshelve":
        return s.show({ kind: "unshelve" });
      case "createpatch":
        return this.createPatch([l.path]);
      case "applypatch":
        return this.applyPatch();
      case "properties":
        return this.openProps(l.path, l.isDir);
    }
  };
}

const key = Symbol("actions");

export function provideActions(a: Actions) {
  setContext(key, a);
}

export function useActions(): Actions {
  return getContext<Actions>(key);
}
