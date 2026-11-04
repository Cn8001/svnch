<script lang="ts">
  import type { LogEntry } from "./svn";

  let {
    entries,
    current,
    loading,
    onmenu,
    onopen,
    onreload,
  }: {
    entries: LogEntry[];
    current: number; // the working copy's revision, marked as where you are
    loading: boolean;
    onmenu: (e: LogEntry, ev: MouseEvent) => void;
    onopen: (e: LogEntry) => void; // double-click: show what the commit changed
    onreload: () => void;
  } = $props();

  let selected = $state(-1);
  let chosen = $derived(entries.find((e) => e.revision === selected));

  function when(date: string): string {
    const d = new Date(date);
    return isNaN(d.getTime()) ? "" : d.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
  }
</script>

<div class="history">
  <div class="list" role="listbox" aria-label="Commits">
    {#each entries as e (e.revision)}
      <div
        class="commit"
        class:sel={selected === e.revision}
        class:ahead={e.revision > current}
        role="option"
        aria-selected={selected === e.revision}
        tabindex="-1"
        onclick={() => (selected = e.revision)}
        ondblclick={() => onopen(e)}
        onkeydown={(ev) => ev.key === "Enter" && onopen(e)}
        oncontextmenu={(ev) => { ev.preventDefault(); selected = e.revision; onmenu(e, ev); }}
      >
        <span class="rev">r{e.revision}</span>
        <span class="msg">{e.message.split("\n")[0] || "(no message)"}</span>
        {#if e.revision === current}<span class="here" title="Your working copy is at this revision">you are here</span>{/if}
        <span class="who muted">{e.author}</span>
        <span class="when muted">{when(e.date)}</span>
      </div>
    {:else}
      <p class="muted empty">{loading ? "Reading history..." : "No commits."}</p>
    {/each}
    {#if entries.length && !loading}
      <div class="foot"><button class="small" onclick={onreload}>Refresh</button></div>
    {/if}
  </div>
  {#if chosen}
    <div class="detail">
      <div><b>r{chosen.revision}</b> by {chosen.author || "unknown"}, {when(chosen.date)}</div>
      <pre class="message">{chosen.message || "(no message)"}</pre>
      <pre class="paths">{#each chosen.paths ?? [] as p}<span class="act {p.action}">{p.action}</span> {p.path}{"\n"}{/each}</pre>
    </div>
  {/if}
</div>

<style>
  .history { display: grid; grid-template-rows: minmax(0, 1fr) auto; height: 100%; min-height: 0; }
  .list { overflow: auto; padding: 4px 0; min-height: 0; }
  .commit {
    display: grid;
    grid-template-columns: 5em minmax(0, 1fr) auto 9em 11em;
    align-items: center;
    gap: 10px;
    height: 26px;
    padding: 0 12px;
    cursor: default;
    white-space: nowrap;
  }
  .commit:hover { background: var(--hover); }
  .commit.sel { background: var(--accent-soft); box-shadow: inset 2px 0 0 var(--accent); }
  .commit.ahead .msg, .commit.ahead .rev { opacity: 0.6; }
  .rev { font-family: var(--mono); font-size: 12px; font-weight: 700; color: var(--blue); }
  .msg { overflow: hidden; text-overflow: ellipsis; }
  .here { padding: 0 7px; border-radius: 9px; background: var(--accent-soft); color: var(--accent); font-size: 11px; font-weight: 700; }
  .commit:not(:has(.here)) { grid-template-columns: 5em minmax(0, 1fr) 9em 11em; }
  .who, .when { overflow: hidden; text-overflow: ellipsis; font-size: 12px; }
  .empty { padding: 16px; margin: 0; }
  .foot { padding: 8px 12px; }
  .small { padding: 1px 10px; font-size: 12px; }
  .detail { display: grid; gap: 8px; padding: 10px 12px; border-top: 1px solid var(--line); background: var(--surface); max-height: 42vh; overflow: auto; }
  .message, .paths { margin: 0; padding: 8px 10px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); white-space: pre-wrap; }
  .act { font-weight: bold; }
  .act.A { color: var(--add-ink); }
  .act.M { color: var(--mod-ink); }
  .act.D { color: var(--del-ink); }
</style>
