<script lang="ts">
  import type { Snippet } from "svelte";

  let {
    title,
    onclose,
    wide = false,
    children,
    footer,
  }: { title: string; onclose: () => void; wide?: boolean; children: Snippet; footer?: Snippet } = $props();
</script>

<svelte:window onkeydown={(e) => e.key === "Escape" && onclose()} />

<div class="backdrop" role="presentation" onclick={onclose}>
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
    background: rgba(36, 33, 30, 0.35);
  }
  .box {
    width: min(480px, calc(100vw - 32px));
    max-height: calc(100vh - 48px);
    display: flex;
    flex-direction: column;
    background: var(--panel);
    border: 2px solid var(--ink);
    box-shadow: 5px 5px 0 var(--ink);
  }
  .box.wide { width: min(860px, calc(100vw - 32px)); }
  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 4px 4px 4px 10px;
    background: var(--ink);
    color: var(--paper);
    font-weight: bold;
  }
  .x { padding: 0 7px; box-shadow: none; border-color: var(--paper); }
  .body { padding: 12px; overflow: auto; display: grid; gap: 10px; }
  footer {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding: 10px 12px;
    border-top: 2px solid var(--ink);
  }
</style>
