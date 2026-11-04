<script lang="ts">
  import { relative, statusCode, type StatusEntry } from "./svn";

  let {
    wc,
    entries,
    checked,
    focused,
    onfocus,
    ontoggle,
    ontoggleall,
    oncontext,
  }: {
    wc: string;
    entries: StatusEntry[];
    checked: Set<string>;
    focused: string;
    onfocus: (path: string) => void;
    ontoggle: (path: string) => void;
    ontoggleall: (on: boolean) => void;
    oncontext: (path: string, e: MouseEvent) => void;
  } = $props();

  let allOn = $derived(entries.length > 0 && entries.every((e) => checked.has(e.path)));
</script>

<div class="list">
  <div class="head">
    <input type="checkbox" checked={allOn} disabled={!entries.length} onchange={() => ontoggleall(!allOn)} />
    <span class="code">St</span>
    <span>Path</span>
    <span class="count muted">{entries.length} {entries.length === 1 ? "change" : "changes"}</span>
  </div>
  <div class="rows">
    {#each entries as e (e.path)}
      <div
        class="row {e.item}"
        class:focused={e.path === focused}
        role="row"
        tabindex="-1"
        onclick={() => onfocus(e.path)}
        onkeydown={(ev) => {
          if (ev.key === "Enter") onfocus(e.path);
          else if (ev.key === " ") { ev.preventDefault(); ontoggle(e.path); }
        }}
        oncontextmenu={(ev) => { ev.preventDefault(); onfocus(e.path); oncontext(e.path, ev); }}
      >
        <input
          type="checkbox"
          checked={checked.has(e.path)}
          onclick={(ev) => ev.stopPropagation()}
          onchange={() => ontoggle(e.path)}
        />
        <span class="code">{statusCode(e)}</span>
        <span class="path" title={e.path}>{relative(wc, e.path)}</span>
      </div>
    {:else}
      <p class="empty muted">No local changes.</p>
    {/each}
  </div>
</div>

<style>
  .list { display: flex; flex-direction: column; min-height: 0; height: 100%; }
  .head, .row {
    display: grid;
    grid-template-columns: 22px 34px 1fr auto;
    align-items: center;
    gap: 6px;
    padding: 2px 8px;
  }
  .head { border-bottom: 2px solid var(--ink); background: var(--cream); font-weight: bold; }
  .count { font-weight: normal; }
  .rows { overflow: auto; flex: 1; }
  .row { cursor: default; border-bottom: 1px dotted rgba(36, 33, 30, 0.18); }
  .row:hover { background: rgba(232, 212, 159, 0.5); }
  .row.focused { background: var(--select); }
  .code { white-space: pre; font-weight: bold; }
  .path { grid-column: span 2; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .added .code, .replaced .code { color: var(--add-ink); }
  .deleted .code, .missing .code, .conflicted .code, .obstructed .code { color: var(--brick); }
  .conflicted .path { color: var(--brick); font-weight: bold; }
  .unversioned .path, .ignored .path { color: var(--muted); }
  .empty { padding: 12px; margin: 0; }
  input[type="checkbox"] { margin: 0; accent-color: var(--ink); }
</style>
