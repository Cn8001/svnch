<script lang="ts">
  import type { MenuItem } from "./svn";

  let { x, y, items, onclose }: { x: number; y: number; items: MenuItem[]; onclose: () => void } = $props();

  let el: HTMLUListElement | undefined = $state();

  // Flip the menu back inside the window when it was opened near the right or bottom edge.
  let pos = $derived.by(() => {
    if (!el) return { left: x, top: y };
    const r = el.getBoundingClientRect();
    return {
      left: Math.min(x, window.innerWidth - r.width - 4),
      top: Math.min(y, window.innerHeight - r.height - 4),
    };
  });
</script>

<svelte:window
  onclick={onclose}
  oncontextmenu={onclose}
  onkeydown={(e) => e.key === "Escape" && onclose()}
  onblur={onclose}
/>

<ul class="ctx" bind:this={el} style:left="{pos.left}px" style:top="{pos.top}px">
  {#each items as item}
    {#if item.separator}
      <li class="sep"></li>
    {:else}
      <li>
        <button
          disabled={item.disabled}
          onclick={(e) => { e.stopPropagation(); onclose(); item.action(); }}
        >{item.label}</button>
      </li>
    {/if}
  {/each}
</ul>

<style>
  .ctx {
    position: fixed;
    z-index: 30;
    min-width: 210px;
    margin: 0;
    padding: 4px;
    list-style: none;
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: 8px;
    box-shadow: var(--shadow);
  }
  button {
    display: block;
    width: 100%;
    border: 0;
    border-radius: 4px;
    background: transparent;
    padding: 4px 10px;
    text-align: left;
  }
  button:hover:not(:disabled) { background: var(--accent); color: var(--accent-ink); }
  .sep { border-top: 1px solid var(--line); margin: 4px 6px; }
</style>
