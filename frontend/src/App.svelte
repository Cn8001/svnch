<script lang="ts">
  import { onMount } from "svelte";
  import MenuBar from "./lib/MenuBar.svelte";
  import FileList from "./lib/FileList.svelte";
  import DiffView from "./lib/DiffView.svelte";
  import HistoryView from "./lib/HistoryView.svelte";
  import BlameView from "./lib/BlameView.svelte";
  import ContextMenu from "./lib/ContextMenu.svelte";
  import SvnMissing from "./lib/SvnMissing.svelte";
  import DialogHost from "./lib/dialogs/DialogHost.svelte";
  import StatusBar from "./lib/StatusBar.svelte";
  import Splitter from "./lib/Splitter.svelte";
  import ProjectBadge from "./lib/ProjectBadge.svelte";
  import { Events } from "@wailsio/runtime";
  import { relative } from "./lib/svn";
  import { Session, provideSession, baseName, parentOf } from "./lib/session.svelte";
  import { Actions, provideActions } from "./lib/actions";
  import { Theme, provideTheme } from "./lib/theme.svelte";
  import { fileItems, groupItems, historyItems, wcItems, projectItems, menuBar } from "./lib/menus";

  const s = new Session();
  const a = new Actions(s);
  const theme = new Theme();
  provideSession(s);
  provideActions(a);
  provideTheme(theme);

  $effect(() => {
    document.documentElement.dataset.theme = theme.light ? "light" : "dark";
  });

  let menus = $derived(menuBar(s, a));

  // The panes' sizes are a per-person preference, kept in this browser storage and nowhere else.
  function stored(key: string, fallback: number): number {
    try {
      const v = Number(localStorage.getItem(key));
      return Number.isFinite(v) && v > 0 ? v : fallback;
    } catch {
      return fallback;
    }
  }
  function keep(key: string, v: number) {
    try {
      localStorage.setItem(key, String(Math.round(v)));
    } catch {}
  }
  let width = $state(window.innerWidth);
  let leftWidth = $state(stored("svnch.leftWidth", Math.round(window.innerWidth * 0.36)));
  let commitHeight = $state(stored("svnch.commitHeight", 150));
  let leftShown = $derived(Math.max(260, Math.min(width - 320, leftWidth)));
  let commitLabel = $derived(s.checked.size ? `Commit ${s.checked.size} selected` : "Commit all changes");

  function popup(e: MouseEvent, items: ReturnType<typeof wcItems>) {
    s.ctxMenu = { x: e.clientX, y: e.clientY, items };
  }

  function below(e: MouseEvent, items: ReturnType<typeof wcItems>) {
    const b = (e.currentTarget as HTMLElement).getBoundingClientRect();
    s.ctxMenu = { x: b.left, y: b.bottom + 4, items };
  }

  function onKey(e: KeyboardEvent) {
    if (s.dialog || s.login.open || s.cert.open || s.ctxMenu) return;
    const mod = e.ctrlKey || e.metaKey;
    if (e.key === "F5") {
      e.preventDefault();
      s.refresh();
    } else if (mod && e.key === "o") {
      e.preventDefault();
      s.openFolder();
    } else if (mod && e.key === "b") {
      e.preventDefault();
      if (!s.svnMissing) a.openRepos();
    } else if (mod && e.key === "u") {
      e.preventDefault();
      a.update();
    } else if (mod && e.key === "Enter") {
      e.preventDefault();
      a.commit();
    }
  }

  onMount(() => {
    a.start();
    return Events.On("svn:progress", (e) => s.onProgress(e.data as { command: string; items: number; item: string }));
  });
</script>

<svelte:window bind:innerWidth={width} onkeydown={onKey} oncontextmenu={(e) => e.preventDefault()} />

<div class="app">
  <MenuBar {menus} />

  <div class="toolbar">
    <button class="project" title={s.wc ? `${s.wc}, click to switch to another working copy` : "Open or switch to a working copy"} onclick={(e) => below(e, projectItems(s, a))} disabled={s.svnMissing}>
      {#if s.wc}
        <ProjectBadge name={baseName(s.wc)} path={s.wc} />
      {:else}
        <svg class="folder" viewBox="0 0 16 16" width="14" height="14"><path d="M1.5 3.5h4.5l1.5 1.5h7v8h-13z" fill="currentColor" /></svg>
      {/if}
      <span class="pname">{s.wc ? baseName(s.wc) : "No working copy"}</span>
      <svg viewBox="0 0 16 16" width="11" height="11"><path d="M4 6l4 4 4-4" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" /></svg>
    </button>
    <span class="sep"></span>
    <button onclick={s.openFolder} disabled={!!s.busy || s.svnMissing}>Open...</button>
    <button onclick={() => a.openRepos()} disabled={!!s.busy || s.svnMissing}>Repos</button>
    <span class="sep"></span>
    <button onclick={() => a.update()} disabled={!s.wc || !!s.busy}>Update</button>
    <button onclick={s.refresh} disabled={!s.wc || !!s.busy}>Refresh</button>
    <button onclick={() => a.openLog(s.wc)} disabled={!s.wc || !!s.busy}>Log</button>
    {#if s.info}
      <div class="where" role="button" tabindex="-1" title="Right-click for details" oncontextmenu={(e) => { e.preventDefault(); popup(e, wcItems(s, a)); }} ondblclick={a.openDetails}>
        <button class="branch" title="{s.info.url}, click to switch branch" onclick={() => s.show({ kind: "switch" })} disabled={!!s.busy}>{s.info.relativeUrl || s.info.url}</button>
        <span class="rev">r{s.info.revision}</span>
      </div>
    {/if}
  </div>

  {#if s.svnMissing}
    <div class="welcome">
      <img src="/appicon.png" alt="" />
      <SvnMissing setup={s.setup} checking={s.checkingSvn} oncheck={s.checkSvn} />
    </div>
  {:else if !s.wc}
    <div class="welcome">
      <img src="/appicon.png" alt="" />
      <p>Open a working copy, or check one out from a repository.</p>
      <div class="actions">
        <button class="primary" onclick={s.openFolder} disabled={!!s.busy}>Open working copy...</button>
        <button onclick={() => a.openCheckout()} disabled={!!s.busy}>Checkout...</button>
        <button onclick={() => a.openRepos()} disabled={!!s.busy}>Browse repositories...</button>
      </div>
      {#if s.recent.length}
        <ul class="recent">
          {#each s.recent as p (p)}
            <li><button onclick={() => s.openWC(p)} disabled={!!s.busy} title={p}><ProjectBadge name={baseName(p)} path={p} size={22} /><b>{baseName(p)}</b><span class="muted">{parentOf(p)}</span></button></li>
          {/each}
        </ul>
      {/if}
    </div>
  {:else}
    <main style:grid-template-columns="{leftShown}px auto minmax(0, 1fr)">
      <section class="left">
        {#if s.conflicts.length}
          <div class="banner">
            <span><b>{s.conflicts.length}</b> {s.conflicts.length === 1 ? "file conflicts" : "files conflict"}. Commit waits until they are resolved.</span>
            <button class="small" onclick={() => s.show({ kind: "conflicts", first: "" })}>Resolve...</button>
          </div>
        {/if}
        <div class="files">
          <FileList
            wc={s.wc}
            entries={s.entries}
            checked={s.checked}
            focused={s.focused}
            onfocus={s.showDiff}
            ontoggle={(p) => (s.checked.has(p) ? s.checked.delete(p) : s.checked.add(p))}
            onlythese={a.selectOnly}
            oncontext={(p, e) => popup(e, fileItems(s, a, p))}
            ongroupcontext={(g, e) => popup(e, groupItems(s, a, g))}
          />
        </div>
        <Splitter vertical after size={commitHeight} min={90} max={480} onresize={(h) => { commitHeight = h; keep("svnch.commitHeight", h); }} />
        <form class="commit" style:height="{commitHeight}px" onsubmit={(e) => { e.preventDefault(); a.commit(); }}>
          <textarea bind:value={s.message} bind:this={s.commitBox} placeholder="Commit message" rows="4"></textarea>
          <button class="primary" type="submit" disabled={!s.canCommit} title="Updates first, then commits">{commitLabel}</button>
        </form>
      </section>
      <Splitter size={leftShown} min={260} max={width - 320} onresize={(w) => { leftWidth = w; keep("svnch.leftWidth", w); }} />
      <section class="right">
        <div class="tabs" role="tablist">
          <button role="tab" aria-selected={s.pane === "diff"} class:on={s.pane === "diff"} onclick={() => s.showPane("diff")}>Diff</button>
          <button role="tab" aria-selected={s.pane === "history"} class:on={s.pane === "history"} onclick={() => s.showPane("history")}>History</button>
          {#if s.blame.target}<button role="tab" aria-selected={s.pane === "blame"} class:on={s.pane === "blame"} onclick={() => s.showPane("blame")}>Blame</button>{/if}
        </div>
        <div class="pane">
          {#if s.pane === "diff"}
            <DiffView title={s.diffTitle} text={s.diffText} loading={s.diffLoading} />
          {:else if s.pane === "blame"}
            <BlameView title={relative(s.wc, s.blame.target)} lines={s.blame.lines} loading={s.blame.loading} onrevision={(rev) => s.showRevisionDiff(s.blame.target, rev)} />
          {:else}
            <HistoryView
              entries={s.history.entries}
              current={s.info?.revision ?? 0}
              loading={s.history.loading}
              onmenu={(e, ev) => popup(ev, historyItems(s, a, e))}
              onopen={(e) => s.showRevisionDiff(s.info?.url ?? s.wc, e.revision)}
              onreload={s.loadHistory}
            />
          {/if}
        </div>
      </section>
    </main>
  {/if}

  <StatusBar />
</div>

{#if s.ctxMenu}
  <ContextMenu x={s.ctxMenu.x} y={s.ctxMenu.y} items={s.ctxMenu.items} onclose={() => (s.ctxMenu = null)} />
{/if}

<DialogHost />

<style>
  .app { display: grid; grid-template-rows: auto auto 1fr auto; grid-template-columns: minmax(0, 1fr); height: 100%; }

  .toolbar { display: flex; align-items: center; gap: 6px; padding: 6px 10px; background: var(--surface); border-bottom: 1px solid var(--line); }
  .toolbar .sep { width: 1px; align-self: stretch; margin: 4px 4px; background: var(--line); }
  .where { display: flex; align-items: center; gap: 10px; margin-left: auto; min-width: 0; cursor: default; padding: 2px 4px 2px 8px; border-radius: var(--radius); }
  .where:hover { background: var(--hover); }
  .where span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .project { display: flex; align-items: center; gap: 6px; padding: 4px 10px; font-weight: 600; max-width: 28ch; min-width: 0; flex: 0 1 auto; }
  .project .folder { flex: none; color: var(--blue); }
  .project .pname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .branch { min-width: 0; border: 0; background: transparent; padding: 1px 6px; color: var(--muted); font-family: var(--mono); font-size: 12px; overflow: hidden; text-overflow: ellipsis; }
  .rev { flex: none; }
  .branch:hover:not(:disabled) { background: var(--hover); color: var(--text); }
  .rev { padding: 1px 8px; border-radius: 10px; background: var(--blue-soft); color: var(--blue); font-family: var(--mono); font-size: 12px; font-weight: 700; }

  .welcome { display: grid; place-content: center; justify-items: center; gap: 14px; text-align: center; }
  .welcome img { width: 120px; height: 120px; }
  .welcome p { margin: 0; }
  .actions { display: flex; gap: 10px; }
  .recent { list-style: none; margin: 8px 0 0; padding: 0; display: grid; gap: 4px; width: min(520px, 90vw); text-align: left; }
  .recent button { display: flex; align-items: center; gap: 10px; width: 100%; border-color: transparent; background: transparent; padding: 6px 10px; }
  .recent button:hover:not(:disabled) { background: var(--hover); border-color: var(--line); }
  .recent span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: var(--mono); font-size: 12px; }

  main { display: grid; min-height: 0; }
  .left { display: flex; flex-direction: column; min-height: 0; min-width: 0; background: var(--surface); }
  .files { flex: 1; min-height: 0; }
  .banner { display: flex; align-items: center; justify-content: space-between; gap: 8px; padding: 6px 10px; background: var(--del-bg); color: var(--del-ink); border-bottom: 1px solid var(--line); font-size: 12px; }
  .banner button { color: var(--text); }
  .commit { display: grid; grid-template-rows: minmax(0, 1fr) auto; gap: 8px; padding: 10px; flex: none; }
  .commit textarea { min-height: 0; }
  .commit button { padding: 6px 12px; }
  .right { display: grid; grid-template-rows: auto 1fr; min-width: 0; min-height: 0; background: var(--bg); }
  .pane { display: grid; min-height: 0; overflow: hidden; }
  .tabs { display: flex; gap: 2px; padding: 4px 8px 0; border-bottom: 1px solid var(--line); background: var(--surface); }
  .tabs button { border: 0; border-bottom: 2px solid transparent; border-radius: 0; background: transparent; color: var(--muted); padding: 5px 12px; font-weight: 600; }
  .tabs button:hover:not(:disabled) { background: transparent; color: var(--text); }
  .tabs button.on { color: var(--text); border-bottom-color: var(--accent); }

</style>
