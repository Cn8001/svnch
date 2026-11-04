<script lang="ts">
  import type { Snippet } from "svelte";

  let {
    title,
    onclose,
    wide = false,
    children,
    footer,
  }: { title: string; onclose: () => void; wide?: boolean; children: Snippet; footer?: Snippet } = $props();

  let backdrop: HTMLDivElement | undefined = $state();

  // The login can open over another window; Escape closes only the one on top.
  function onkey(e: KeyboardEvent) {
    if (e.key !== "Escape") return;
    const all = document.querySelectorAll(".backdrop");
    if (all[all.length - 1] === backdrop) onclose();
  }
</script>

<svelte:window onkeydown={onkey} />

<div class="backdrop" role="presentation" onclick={onclose} bind:this={backdrop}>
  <!-- Escape is handled on the window, so the click guard needs no key handler of its own. -->
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="box" class:wide role="dialog" tabindex="-1" aria-label={title} onclick={(e) => e.stopPropagation()}>
    <header>
      <span>{title}</span>
      <button class="x" aria-label="Close" onclick={onclose}>x</button>
    </header>
    <div class="body">{@render children()}</div>
    {#if footer}<footer>{@render footer()}</footer>{/if}
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 40;
    display: grid;
    place-items: center;
    background: var(--backdrop);
  }
  .box {
    width: min(480px, calc(100vw - 32px));
    max-height: calc(100vh - 48px);
    display: flex;
    flex-direction: column;
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: 10px;
    box-shadow: var(--shadow);
    overflow: hidden;
  }
  .box.wide { width: min(860px, calc(100vw - 32px)); }
  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 10px 10px 16px;
    border-bottom: 1px solid var(--line);
    font-weight: 600;
  }
  .x { width: 26px; height: 26px; padding: 0; border-color: transparent; background: transparent; color: var(--muted); font-size: 15px; line-height: 1; }
  .x:hover:not(:disabled) { background: var(--hover); border-color: transparent; color: var(--text); }
  /* max-content rows keep each block its own height and let the whole body scroll, instead of squeezing a scrolling child to nothing */
  .body { padding: 14px 16px; overflow: auto; display: grid; grid-auto-rows: max-content; gap: 12px; }
  footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 12px 16px;
    border-top: 1px solid var(--line);
    background: var(--bg);
  }
</style>
