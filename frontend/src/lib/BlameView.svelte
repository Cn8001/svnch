<script lang="ts">
  import type { BlameLine } from "./svn";

  let {
    title,
    lines,
    loading,
    onrevision,
  }: {
    title: string;
    lines: BlameLine[];
    loading: boolean;
    onrevision: (rev: number) => void; // click on a revision: show what that commit changed
  } = $props();

  // Consecutive lines from one commit share a block; the gutter only names the commit at the block's first line.
  let shown = $derived(lines.map((l, i) => ({ ...l, first: i === 0 || lines[i - 1].revision !== l.revision })));

  function day(date: string): string {
    const d = new Date(date);
    return isNaN(d.getTime()) ? "" : d.toLocaleDateString();
  }
</script>

<div class="blame">
  <div class="head"><span class="title">{title}</span><span class="muted">{lines.length ? `${lines.length} lines` : ""}</span></div>
  <div class="body">
    {#if loading}
      <p class="muted empty">Reading who changed each line...</p>
    {:else if !lines.length}
      <p class="muted empty">Nothing to show.</p>
    {:else}
      <pre>{#each shown as l (l.line)}<span class="line" class:first={l.first}><span class="who">{#if l.first}{#if l.revision}<button class="rev" onclick={() => onrevision(l.revision)} title="Show what r{l.revision} changed">r{l.revision}</button> {l.author} <span class="muted">{day(l.date)}</span>{:else}<span class="muted">local change</span>{/if}{/if}</span><span class="num">{l.line}</span><span class="txt">{l.text || " "}</span></span>{/each}</pre>
    {/if}
  </div>
</div>

<style>
  .blame { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  .head { display: flex; justify-content: space-between; gap: 8px; align-items: center; height: 34px; padding: 0 12px; border-bottom: 1px solid var(--line); font-weight: 600; }
  .title { font-family: var(--mono); font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .body { flex: 1; overflow: auto; background: var(--field); }
  pre { min-width: max-content; line-height: 1.55; }
  .line { display: grid; grid-template-columns: 24ch 5ch 1fr; }
  .line.first { border-top: 1px solid var(--line); }
  .who { padding: 0 8px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 11px; color: var(--text); background: var(--surface); }
  .rev { border: 0; padding: 0; background: transparent; color: var(--blue); font: inherit; font-weight: 700; cursor: pointer; }
  .rev:hover:not(:disabled) { background: transparent; text-decoration: underline; }
  .num { padding-right: 8px; text-align: right; color: var(--muted); opacity: 0.7; border-right: 1px solid var(--line); user-select: none; }
  .txt { padding-left: 10px; padding-right: 12px; white-space: pre; }
  .empty { padding: 16px; margin: 0; }
</style>
