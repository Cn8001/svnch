<script lang="ts">
  import { initials, badgeColor } from "./svn";

  let { name, path, size = 18 }: { name: string; path: string; size?: number } = $props();
</script>

<span class="badge" style:--size="{size}px" style:background={badgeColor(path)} aria-hidden="true">{initials(name)}</span>

<style>
  .badge {
    flex: none;
    display: inline-grid;
    place-items: center;
    width: var(--size);
    height: var(--size);
    border-radius: 4px;
    color: #fff;
    font-size: calc(var(--size) * 0.5);
    font-weight: 700;
    letter-spacing: 0.02em;
    line-height: 1;
    text-shadow: 0 1px 0 rgba(0, 0, 0, 0.2);
  }
</style>
