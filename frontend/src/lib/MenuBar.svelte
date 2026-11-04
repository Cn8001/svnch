<script lang="ts">
  import type { MenuItem } from "./svn";

  type Menu = { title: string; items: MenuItem[] } | { title: string; action: () => void };

  let { menus }: { menus: Menu[] } = $props();

  let open = $state(-1);

  function pick(item: MenuItem) {
    open = -1;
    if (!item.separator && !item.disabled) item.action();
  }
</script>

<svelte:window
  onclick={() => (open = -1)}
  onkeydown={(e) => e.key === "Escape" && (open = -1)}
/>

<nav class="bar">
  <span class="brand"><img src="/appicon.png" alt="" />svnch</span>
  {#each menus as menu, i}
    <div class="menu">
      {#if "action" in menu}
        <button class="title" onclick={(e) => { e.stopPropagation(); open = -1; menu.action(); }}>{menu.title}</button>
      {:else}
        <button
          class="title"
          class:active={open === i}
          onclick={(e) => { e.stopPropagation(); open = open === i ? -1 : i; }}
          onmouseenter={() => open !== -1 && (open = i)}
        >{menu.title}</button>
      {/if}
      {#if open === i && "items" in menu}
        <ul class="drop">
          {#each menu.items as item}
            {#if item.separator}
              <li class="sep"></li>
            {:else}
              <li>
                <button disabled={item.disabled} onclick={(e) => { e.stopPropagation(); pick(item); }}>
                  <span>{item.label}</span>
                  {#if item.hint}<span class="hint">{item.hint}</span>{/if}
                </button>
              </li>
            {/if}
          {/each}
        </ul>
      {/if}
    </div>
  {/each}
</nav>

<style>
  .bar {
    display: flex;
    align-items: stretch;
    background: var(--cream);
    border-bottom: 2px solid var(--ink);
    height: 30px;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 0 12px 0 8px;
    font-weight: bold;
    border-right: 2px solid var(--ink);
  }
  .brand img { width: 18px; height: 18px; }
  .menu { position: relative; }
  .title {
    height: 100%;
    border: 0;
    box-shadow: none;
    background: transparent;
    padding: 0 12px;
  }
  .title.active, .title:hover { background: var(--ink); color: var(--paper); }
  .title:active { transform: none; }
  .drop {
    position: absolute;
    top: 100%;
    left: 0;
    z-index: 20;
    min-width: 220px;
    margin: 0;
    padding: 3px 0;
    list-style: none;
    background: var(--panel);
    border: 2px solid var(--ink);
    box-shadow: 3px 3px 0 var(--ink);
  }
  .drop button {
    display: flex;
    justify-content: space-between;
    gap: 24px;
    width: 100%;
    border: 0;
    box-shadow: none;
    background: transparent;
    padding: 3px 12px;
    text-align: left;
  }
  .drop button:hover:not(:disabled) { background: var(--ink); color: var(--paper); }
  .drop button:active { transform: none; }
  .hint { color: var(--muted); }
  .drop button:hover .hint { color: var(--cream); }
  .sep { border-top: 1px solid var(--muted); margin: 3px 8px; }
</style>
