import { Application, Browser, Clipboard } from "@wailsio/runtime";
import { hasChanges, isVersioned, type LogEntry, type MenuItem, type StatusEntry } from "./svn";
import { baseName, parentOf, type Session } from "./session.svelte";
import type { Actions } from "./actions";
import type { Group } from "./FileList.svelte";

export const REPO_URL = "https://github.com/Cn8001/svnch";

// fileItems is the right-click menu of a row in the changes tree; on a checked row it acts on every checked row.
export function fileItems(s: Session, a: Actions, path: string): MenuItem[] {
  const e = s.entry(path);
  const paths = s.targetsFor(path);
  const sel = paths.map(s.entry).filter((x): x is StatusEntry => !!x);
  const many = paths.length > 1;
  const n = many ? ` (${paths.length})` : "";
  const folder = !e; // a folder row in the tree that only holds changes; svn lists it only when it changed itself
  const versioned = folder || isVersioned(e);
  const unversioned = sel.filter((x) => !isVersioned(x)).map((x) => x.path);
  const conflicted = !!e && (e.item === "conflicted" || e.treeConflicted || e.props === "conflicted");
  return [
    { label: "Show diff", action: () => s.showDiff(path), disabled: many || !versioned },
    { separator: true },
    { label: "Add" + (unversioned.length > 1 ? ` (${unversioned.length})` : ""), action: () => a.add(unversioned), disabled: !unversioned.length },
    ...(conflicted ? [{ label: "Resolve conflict...", action: () => s.show({ kind: "conflicts", first: path }) }] : []),
    { label: "Discard changes..." + n, action: () => a.discard(paths), disabled: !s.under(paths).length },
    ...(unversioned.length ? [{ label: "Add to ignore list" + (unversioned.length > 1 ? ` (${unversioned.length})` : ""), action: () => a.ignore(unversioned) }] : []),
    { label: "Delete..." + n, action: () => a.remove(paths, false), disabled: unversioned.length > 0 },
    { label: "Remove from svn, keep file..." + n, action: () => a.remove(paths, true), disabled: unversioned.length > 0 },
    { label: "Rename / move...", action: () => s.show({ kind: "move", path }), disabled: many || !versioned },
    { separator: true },
    { label: "Move to changelist..." + n, action: () => s.show({ kind: "changelist", paths: sel.filter(isVersioned).map((x) => x.path) }), disabled: !sel.some(isVersioned) },
    { label: "Remove from changelist", action: () => a.leaveChangelist(sel.filter((x) => x.changelist).map((x) => x.path)), disabled: !sel.some((x) => x.changelist) },
    { separator: true },
    { label: "Show log", action: () => a.openLog(path), disabled: many || !versioned || e?.item === "added" || e?.item === "ignored" },
    { label: "Diff with previous version", action: () => a.diffPrevious(path), disabled: many || !versioned || folder || e?.item === "added" },
    { label: "Blame", action: () => s.openBlame(path), disabled: many || !versioned || folder || e?.item === "added" },
    { separator: true },
    e?.locked
      ? { label: "Release lock", action: () => a.unlock(sel.filter((x) => x.locked).map((x) => x.path)) }
      : { label: "Get lock..." + n, action: () => s.show({ kind: "lock", paths }), disabled: !versioned || folder },
    { label: "Create patch..." + n, action: () => a.createPatch(paths), disabled: !sel.some(hasChanges) && !folder },
    ...(versioned && !folder && e?.item !== "added" ? [{ label: "Unversion and add to ignore list" + n, action: () => a.unversionIgnore(paths) }] : []),
    { label: "Properties...", action: () => a.openProps(path), disabled: many || !versioned },
    { label: "Copy path", action: () => Clipboard.SetText(path) },
  ];
}

// groupItems is the right-click menu of a group's own row in the changes tree, acting on everything in that group.
export function groupItems(s: Session, a: Actions, g: Group): MenuItem[] {
  const n = ` (${g.paths.length})`;
  const commitThese = {
    label: "Commit these..." + n,
    action: () => {
      a.selectOnly(g.paths);
      if (g.kind === "changelist" && !s.message.trim()) s.message = `${g.name}: `;
      s.commitBox?.focus();
    },
  };
  const selectThese = { label: "Select only these", action: () => a.selectOnly(g.paths) };
  switch (g.kind) {
    case "unversioned":
      return [
        { label: "Add all" + n, action: () => a.add(g.paths) },
        { label: "Add all and commit..." + n, action: async () => { await a.add(g.paths); commitThese.action(); } },
        { separator: true },
        { label: "Add all to ignore list" + n, action: () => a.ignore(g.paths) },
        { label: "Delete all from disk..." + n, action: () => a.discard(g.paths) },
      ];
    case "ignored":
      return [{ label: "Add anyway" + n, action: () => a.add(g.paths) }];
    default:
      return [
        commitThese,
        selectThese,
        { separator: true },
        { label: "Discard changes..." + n, action: () => a.discard(g.paths) },
        { label: "Create patch..." + n, action: () => a.createPatch(g.paths) },
        { separator: true },
        ...(g.kind === "changelist"
          ? [
              { label: "Rename changelist...", action: () => s.show({ kind: "changelist", paths: g.paths }) },
              { label: "Dissolve changelist", action: () => a.leaveChangelist(g.paths) },
            ]
          : [{ label: "Move to changelist..." + n, action: () => s.show({ kind: "changelist", paths: g.paths.filter((p) => { const e = s.entry(p); return e && isVersioned(e); }) }) }]),
      ];
  }
}

export function historyItems(s: Session, a: Actions, e: LogEntry): MenuItem[] {
  const here = s.info?.revision === e.revision;
  return [
    { label: here ? "You are at this revision" : `Go to r${e.revision}`, action: () => a.updateToRevision(e, s.wc), disabled: here || !!s.busy },
    { label: "Show changes", action: () => s.showRevisionDiff(s.info?.url ?? s.wc, e.revision) },
    { label: "Undo this commit...", action: () => a.undoRevision(e, s.wc), disabled: !!s.busy },
    { separator: true },
    { label: "Copy revision number", action: () => Clipboard.SetText(String(e.revision)) },
    { label: "Copy message", action: () => Clipboard.SetText(e.message) },
  ];
}

// wcItems is the right-click menu of the working copy's name and branch in the toolbar.
export function wcItems(s: Session, a: Actions): MenuItem[] {
  return [
    { label: "Details...", action: a.openDetails },
    { separator: true },
    { label: "Properties...", action: () => a.openProps(s.wc, true) },
    { label: "Copy URL", action: () => s.info && Clipboard.SetText(s.info.url) },
    { label: "Copy repository root", action: () => s.info && Clipboard.SetText(s.info.repositoryRoot) },
    { label: "Copy working copy path", action: () => Clipboard.SetText(s.wc) },
    { separator: true },
    { label: "Browse on server...", action: () => s.info && a.openRepos(s.info.url), disabled: !!s.busy },
    { label: "Switch branch...", action: () => s.show({ kind: "switch" }), disabled: !!s.busy },
    { label: "New branch...", action: () => s.show({ kind: "branch" }), disabled: !!s.busy },
    { label: "Show log", action: () => a.openLog(s.wc), disabled: !!s.busy },
    { label: "Update", action: () => a.update(), disabled: !!s.busy },
    { label: "Log in...", action: () => s.startLogin(s.info?.repositoryRoot) },
  ];
}

// projectItems is the switcher behind the working copy's name in the toolbar.
export function projectItems(s: Session, a: Actions): MenuItem[] {
  return [
    ...s.recent.map((p) => ({ label: baseName(p), hint: parentOf(p), action: () => s.openWC(p), disabled: p === s.wc || !!s.busy })),
    ...(s.recent.length ? [{ separator: true as const }] : []),
    { label: "Open working copy...", action: s.openFolder, disabled: !!s.busy },
    { label: "Checkout...", action: () => a.openCheckout(), disabled: !!s.busy },
    ...(s.recent.length > 1 ? [{ label: "Clear this list", action: () => s.saveRecent(s.wc ? [s.wc] : []) }] : []),
  ];
}

export function menuBar(s: Session, a: Actions) {
  const busy = !!s.busy;
  const noWC = !s.wc || busy;
  return [
    {
      title: "File",
      items: [
        { label: "Open working copy...", action: s.openFolder, hint: "Ctrl+O", disabled: busy },
        { label: "Checkout...", action: () => a.openCheckout(), disabled: busy || s.svnMissing },
        { label: "Browse repositories...", action: () => a.openRepos(), hint: "Ctrl+B", disabled: busy || s.svnMissing },
        { separator: true as const },
        { label: "Import a folder...", action: async () => { const d = await a.pickFolder("Import which folder", "Import"); if (d) s.show({ kind: "import", path: d }); }, disabled: busy || s.svnMissing },
        { label: "Create repository...", action: async () => { const d = await a.pickFolder("Create the repository in", "Create here", true); if (d) a.createRepo(d); }, disabled: busy || s.svnMissing },
        { separator: true as const },
        { label: "Quit", action: () => Application.Quit() },
      ],
    },
    {
      title: "Repository",
      items: [
        { label: "Refresh", action: s.refresh, hint: "F5", disabled: noWC },
        { label: "Update", action: () => a.update(), hint: "Ctrl+U", disabled: noWC },
        { label: "Commit", action: a.commit, hint: "Ctrl+Enter", disabled: !s.canCommit },
        { label: "Resolve conflicts...", action: () => s.show({ kind: "conflicts", first: "" }), disabled: !s.conflicts.length },
        { label: "Properties...", action: () => a.openProps(s.wc, true), disabled: !s.wc },
        { label: "Properties of a file or folder...", action: a.pickProps, disabled: !s.wc },
        { separator: true as const },
        { label: "Clean up", action: a.cleanup, disabled: noWC },
        { label: "Relocate...", action: () => s.show({ kind: "relocate" }), disabled: noWC },
        { label: "Upgrade working copy", action: async () => { if (await a.upgrade()) await s.refresh(); }, disabled: noWC },
        { label: "Export...", action: () => a.exportCopy(), disabled: noWC },
        { label: "Create patch...", action: () => a.createPatch([s.wc]), disabled: noWC || !s.entries.some(hasChanges) },
        { label: "Apply patch...", action: a.applyPatch, disabled: noWC },
        { label: "Discard all changes...", action: () => a.discard([s.wc]), disabled: noWC || !s.entries.some((e) => e.item !== "ignored") },
        { separator: true as const },
        { label: "Details...", action: a.openDetails, disabled: !s.wc },
        { label: "Show log", action: () => a.openLog(s.wc), disabled: noWC },
        { label: "Undo a commit...", action: () => a.openLog(s.wc, "Pick the commit to undo, then press Undo this revision."), disabled: noWC },
        { label: "Merge...", action: () => s.show({ kind: "merge" }), disabled: noWC },
        { separator: true as const },
        { label: "New branch...", action: () => s.show({ kind: "branch" }), disabled: noWC },
        { label: "Switch branch...", action: () => s.show({ kind: "switch" }), disabled: noWC },
        { separator: true as const },
        { label: "Shelve changes...", action: () => s.show({ kind: "shelve" }), disabled: noWC },
        { label: "Unshelve...", action: () => s.show({ kind: "unshelve" }), disabled: noWC },
      ],
    },
    {
      title: "Help",
      items: [
        { label: "svn command reference", action: () => s.show({ kind: "help", topic: "" }) },
        { label: "Installing svn", action: () => { s.checkSvn(); s.show({ kind: "svn" }); } },
        { label: "Report a problem...", action: () => Browser.OpenURL(`${REPO_URL}/issues`) },
        { label: "svnch on GitHub", action: () => Browser.OpenURL(REPO_URL) },
      ],
    },
    { title: "Settings", action: () => s.show({ kind: "settings" }) },
    { title: "About", action: () => s.show({ kind: "about" }) },
  ];
}
