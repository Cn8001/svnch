<script lang="ts">
  import { onMount } from "svelte";
  import Modal from "../Modal.svelte";
  import { svn } from "../svn";
  import { failed, useSession } from "../session.svelte";

  const s = useSession();
  let url = $state(s.info?.url ?? "");
  let choices = $state<{ label: string; url: string }[]>([]);
  let loading = $state(true);

  // The list is what the server really has: trunk when there is one, otherwise the folder the working copy came from
  // (the whole repository when it has no layout), then each branch. Each listing gets CheckConnection's ten seconds,
  // because Commit and Switch wait while a command runs.
  onMount(async () => {
    const l = s.layout();
    const i = s.info;
    if (!l || !i) return;
    const root = i.repositoryRoot.replace(/\/+$/, "");
    const top = await s.run("Reading branches", () => svn.CheckConnection(l.base), l.base);
    const dirs = top === failed ? [] : (top ?? []).filter((e) => e.kind === "dir").map((e) => e.name);
    if (dirs.includes("trunk")) choices.push({ label: "trunk", url: `${l.base}/trunk` });
    else choices.push({ label: l.base === root ? "whole repository (^/)" : `^${l.base.slice(root.length)}`, url: l.base });
    if (dirs.includes("branches")) {
      const r = await s.run("Reading branches", () => svn.CheckConnection(`${l.base}/branches`), l.base);
      if (r !== failed) {
        for (const e of r ?? []) if (e.kind === "dir") choices.push({ label: `branches/${e.name}`, url: `${l.base}/branches/${encodeURIComponent(e.name)}` });
      }
    }
    loading = false;
  });

  function current(u: string): boolean {
    const here = s.info?.url ?? "";
    return here === u || here.startsWith(u + "/");
  }

  async function submit(to: string) {
    const l = s.layout();
    if (!l || !to.trim()) return;
    const target = choices.some((c) => c.url === to) ? to + l.sub : to.trim();
    s.close();
    const rev = await s.run("Switching", () => svn.Switch(s.wc, target), l.base);
    if (rev === failed) return;
    await s.reloadInfo();
    await s.refresh();
    s.notice = `Switched to ${s.info?.relativeUrl || target}, at revision ${rev}.`;
  }
</script>

<Modal title="Switch branch" onclose={s.close}>
  <p class="muted">Points this working copy at another branch. Your local changes come along; files that differ are updated.</p>
  {#if !s.layout()?.line && !loading && choices.length <= 1}
    <p class="muted fine">
      {s.atRoot ? "This working copy is the whole repository, which has no trunk and branches folders" : "This repository has no trunk and branches folders"},
      so there is nothing else to list. A url still works.
    </p>
  {/if}
  {#if choices.length || loading}
    <ul class="choices">
      {#each choices as c (c.url)}
        <li>
          <button class:sel={current(c.url)} onclick={() => submit(c.url)} disabled={!!s.busy && !loading}>
            <span>{c.label}</span>
            {#if current(c.url)}<span class="muted">current</span>{/if}
          </button>
        </li>
      {/each}
      {#if loading}<li class="muted loadrow">Reading branches... <button class="small" onclick={() => s.cancelBusy?.()}>Stop</button></li>{/if}
    </ul>
  {/if}
  <label><span>Or any url</span><input bind:value={url} class="mono" /></label>
  {#snippet footer()}
    <button onclick={s.close}>Cancel</button>
    <button class="primary" disabled={!url.trim() || url === s.info?.url || !!s.busy} onclick={() => submit(url)}>Switch to url</button>
  {/snippet}
</Modal>

<style>
  .choices { margin: 0; padding: 4px; list-style: none; border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); max-height: 40vh; overflow: auto; }
  .choices button { display: flex; justify-content: space-between; width: 100%; border: 0; border-radius: 4px; background: transparent; text-align: left; padding: 5px 8px; font-family: var(--mono); font-size: 12px; }
  .choices button:hover:not(:disabled) { background: var(--hover); }
  .choices button.sel { background: var(--accent-soft); box-shadow: inset 2px 0 0 var(--accent); }
  .choices li.muted { padding: 5px 8px; }
  .choices .loadrow { display: flex; align-items: center; justify-content: space-between; }
  .choices .loadrow button { width: auto; font-family: inherit; border: 1px solid var(--line-strong); }
</style>
