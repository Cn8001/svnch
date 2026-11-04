<script lang="ts">
  import Modal from "../Modal.svelte";
  import { svn } from "../svn";
  import { failed, useSession } from "../session.svelte";

  const s = useSession();
  let mg = $state({ source: "^/trunk", change: "", dryRun: true, output: "" });

  async function submit() {
    const change = Number(mg.change || 0);
    if (!mg.source || !Number.isInteger(change) || change < 0) return;
    const out = await s.run(mg.dryRun ? "Trying merge" : "Merging", () => svn.Merge({ source: mg.source.trim(), target: s.wc, change, dryRun: mg.dryRun }));
    if (out === failed) return;
    mg.output = out.trim() || "Nothing to merge.";
    if (!mg.dryRun) await s.refresh();
  }
</script>

<Modal title="Merge into working copy" wide onclose={s.close}>
  <div class="row">
    <label class="grow"><span>From (URL or ^/path)</span><input bind:value={mg.source} /></label>
    <label><span>Only revision (empty for all)</span><input bind:value={mg.change} inputmode="numeric" /></label>
  </div>
  <label class="check"><input type="checkbox" bind:checked={mg.dryRun} /> Dry run, change nothing</label>
  {#if mg.output}<pre class="output">{mg.output}</pre>{/if}
  {#snippet footer()}
    <button onclick={s.close}>Close</button>
    <button class="primary" disabled={!mg.source || !!s.busy} onclick={submit}>{mg.dryRun ? "Try merge" : "Merge"}</button>
  {/snippet}
</Modal>
