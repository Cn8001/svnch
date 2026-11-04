<script lang="ts">
  import { onMount } from "svelte";
  import type { MenuItem } from "./svn";
  import ProjectBadge from "./ProjectBadge.svelte";

  let { x, y, items, onclose }: { x: number; y: number; items: MenuItem[]; onclose: () => void } = $props();

  let el: HTMLUListElement | undefined = $state();

  // The click or right-click that opened the menu is still travelling up to the window when the menu mounts, and
  // would close it at once; the window listeners only count once that event is over.
  let armed = false;
  onMount(() => {
    const t = setTimeout(() => (armed = true));
    return () => clearTimeout(t);
  });

  function close() {
    if (armed) onclose();
  }

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
  onclick={close}
  oncontextmenu={close}
  onkeydown={(e) => e.key === "Escape" && onclose()}
  onblur={close}
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
        ><span class="main">{#if item.badge}<ProjectBadge name={item.badge.name} path={item.badge.path} size={16} />{/if}<span>{item.label}</span></span>{#if item.hint}<span class="hint">{item.hint}</span>{/if}</button>
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
    display: flex;
    justify-content: space-between;
    gap: 24px;
    width: 100%;
    border: 0;
    border-radius: 4px;
    background: transparent;
    padding: 4px 10px;
    text-align: left;
  }
  button:hover:not(:disabled) { background: var(--accent); color: var(--accent-ink); }
  .main { display: flex; align-items: center; gap: 8px; min-width: 0; }
  .hint { color: var(--muted); font-size: 12px; max-width: 40ch; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  button:hover:not(:disabled) .hint { color: inherit; opacity: 0.75; }
  .sep { border-top: 1px solid var(--line); margin: 4px 6px; }
</style>
