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
    gap: 2px;
    padding: 0 6px;
    background: var(--surface);
    border-bottom: 1px solid var(--line);
    height: 32px;
  }
  .brand {
    display: flex;
    align-items: center;
    gap: 7px;
    padding: 0 12px 0 4px;
    font-weight: 700;
    letter-spacing: 0.02em;
  }
  .brand img { width: 18px; height: 18px; border-radius: 4px; }
  .menu { position: relative; display: flex; align-items: center; }
  .title {
    height: 24px;
    border: 0;
    background: transparent;
    padding: 0 10px;
    color: var(--muted);
  }
  .title.active, .title:hover:not(:disabled) { background: var(--hover); color: var(--text); }
  .drop {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    z-index: 20;
    min-width: 230px;
    margin: 0;
    padding: 4px;
    list-style: none;
    background: var(--surface);
    border: 1px solid var(--line-strong);
    border-radius: 8px;
    box-shadow: var(--shadow);
  }
  .drop button {
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
  .drop button:hover:not(:disabled) { background: var(--accent); color: var(--accent-ink); }
  .hint { color: var(--muted); font-size: 12px; }
  .drop button:hover:not(:disabled) .hint { color: inherit; opacity: 0.75; }
  .sep { border-top: 1px solid var(--line); margin: 4px 6px; }
</style>
