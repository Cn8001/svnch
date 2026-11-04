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
    min-width: 200px;
    margin: 0;
    padding: 3px 0;
    list-style: none;
    background: var(--panel);
    border: 2px solid var(--ink);
    box-shadow: 3px 3px 0 var(--ink);
  }
  button {
    display: block;
    width: 100%;
    border: 0;
    box-shadow: none;
    background: transparent;
    padding: 3px 12px;
    text-align: left;
  }
  button:hover:not(:disabled) { background: var(--ink); color: var(--paper); }
  button:active { transform: none; }
  .sep { border-top: 1px solid var(--muted); margin: 3px 8px; }
</style>
