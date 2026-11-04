<script lang="ts">
  import { onMount } from "svelte";
  import Modal from "../Modal.svelte";
  import { svn, relative, type LogEntry } from "../svn";
  import { failed, useSession } from "../session.svelte";
  import { useActions } from "../actions";

  let { target, purpose = "" }: { target: string; purpose?: string } = $props();
  const s = useSession();
  const a = useActions();
  let entries = $state<LogEntry[]>([]);
  let loading = $state(true);
  let selected = $state(0);
  let chosen = $derived(entries[selected]);
  let local = $derived(!!s.wc && !/^[a-z][a-z0-9+.-]*:\/\//i.test(target) && !target.startsWith("^/"));

  onMount(async () => {
    const r = await s.run("Reading log", () => svn.Log({ target, limit: 200, verbose: true }), target);
    loading = false;
    if (r !== failed) entries = r ?? [];
  });
</script>

<Modal title="Log: {relative(s.wc, target)}" wide onclose={s.close}>
  {#if purpose}<p class="purpose">{purpose}</p>{/if}
  <div class="log">
    <ul class="revs">
      {#each entries as e, i}
        <li>
          <button class:sel={i === selected} onclick={() => (selected = i)}>
            <b>r{e.revision}</b> <span class="muted">{e.author}</span>
            <span class="msg">{e.message.split("\n")[0] || "(no message)"}</span>
          </button>
        </li>
      {:else}
        <li class="muted">{loading ? "Reading log..." : "No revisions."}</li>
      {/each}
    </ul>
    {#if chosen}
      <div class="detail">
        <div><b>r{chosen.revision}</b> by {chosen.author || "unknown"}, {new Date(chosen.date).toLocaleString()}</div>
        <pre class="output">{chosen.message || "(no message)"}</pre>
        <pre class="paths">{#each chosen.paths ?? [] as p}<span class="act {p.action}">{p.action}</span> {p.path}{"\n"}{/each}</pre>
      </div>
    {/if}
  </div>
  {#snippet footer()}
    {#if local}
      <button disabled={!chosen || !!s.busy} onclick={() => a.updateToRevision(chosen, target)}>Update to this revision</button>
      <button disabled={!chosen || !!s.busy} onclick={() => a.undoRevision(chosen, target)}>Undo this revision...</button>
      <span class="grow"></span>
    {/if}
    <button onclick={s.close}>Close</button>
    <button class="primary" disabled={!chosen} onclick={() => { const rev = chosen.revision; s.close(); s.showRevisionDiff(target, rev); }}>Show diff</button>
  {/snippet}
</Modal>

<style>
  .log { display: grid; grid-template-columns: minmax(240px, 40%) 1fr; gap: 12px; height: 55vh; }
  .revs { margin: 0; padding: 4px; list-style: none; overflow: auto; border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); }
  .revs button { display: block; width: 100%; border: 0; border-radius: 4px; background: transparent; text-align: left; padding: 5px 8px; }
  .revs button:hover:not(:disabled) { background: var(--hover); }
  .revs button.sel { background: var(--accent-soft); box-shadow: inset 2px 0 0 var(--accent); }
  .revs .msg { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .detail { display: grid; grid-template-rows: auto auto 1fr; gap: 8px; min-height: 0; }
  .paths { overflow: auto; padding: 10px; border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); }
  .act { font-weight: bold; }
  .act.A { color: var(--add-ink); }
  .act.M { color: var(--mod-ink); }
  .act.D { color: var(--del-ink); }
</style>
