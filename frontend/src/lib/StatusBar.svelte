<script lang="ts">
  import { useSession } from "./session.svelte";
  import { useActions } from "./actions";

  const s = useSession();
  const a = useActions();

  let p = $derived(s.progress);
  let share = $derived(p.total ? Math.min(1, p.done / p.total) : 0);
  // How long is left, from the pace so far; only once enough is done for the pace to mean something.
  let left = $derived.by(() => {
    if (!p.total || p.done < 5 || p.done >= p.total) return "";
    const ms = ((Date.now() - p.started) / p.done) * (p.total - p.done);
    if (ms < 60_000) return `about ${Math.max(1, Math.round(ms / 1000))} s left`;
    return `about ${Math.round(ms / 60_000)} min left`;
  });

  function n(x: number): string {
    return x.toLocaleString();
  }
</script>

<!-- Above the modal backdrop, so Cancel is reachable while a window is open. -->
<footer class="status">
  {#if s.busy}
    <span class="label">{s.busy}...</span>
    {#if p.done || p.total}
      <div class="bar" class:counting={!p.total} role="progressbar" aria-valuemin={0} aria-valuemax={p.total || undefined} aria-valuenow={p.done}>
        <div class="fill" style:width="{share * 100}%"></div>
      </div>
      <span class="count">{n(p.done)}{p.total ? ` of ${n(Math.max(p.total, p.done))}` : ""} {p.done === 1 && !p.total ? "item" : "items"}{left ? ` · ${left}` : ""}</span>
      {#if p.item}<span class="item" title={p.item}>{p.item}</span>{/if}
    {/if}
    {#if !p.item}<span class="grow"></span>{/if}
    {#if s.cancelBusy}<button class="small" onclick={s.cancelBusy}>Cancel</button>{/if}
  {:else if s.error}
    <button class="link error" title="Show the full message" onclick={a.showError}>{s.error}</button>
  {:else}
    <span>{s.notice || (s.wc ? "Ready." : "")}</span>
  {/if}
</footer>

<style>
  .status { position: relative; z-index: 45; display: flex; align-items: center; gap: 10px; min-height: 26px; padding: 2px 10px; border-top: 1px solid var(--line); background: var(--surface); font-size: 12px; color: var(--muted); }
  .status span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  /* The label gives way first, so the count and Cancel always show. */
  .label { flex: 0 1 auto; min-width: 4em; max-width: 40%; }
  .status > button { flex: none; }
  .count { flex: none; font-variant-numeric: tabular-nums; }
  .item { flex: 1 1 0; min-width: 0; font-family: var(--mono); font-size: 11px; opacity: 0.8; }
  .bar { flex: none; position: relative; width: 180px; height: 6px; border-radius: 3px; background: var(--line); overflow: hidden; }
  .fill { height: 100%; background: var(--accent); border-radius: 3px; transition: width 0.15s; }
  /* Without a total the bar can only say that work goes on. */
  .bar.counting .fill { position: absolute; width: 30% !important; animation: slide 1.2s ease-in-out infinite; }
  @keyframes slide { from { left: -30%; } to { left: 100%; } }
  .link { border: 0; background: transparent; padding: 0; text-align: left; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .link:hover:not(:disabled) { background: transparent; text-decoration: underline; }
</style>
