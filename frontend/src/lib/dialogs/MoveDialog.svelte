<script lang="ts">
  import { untrack } from "svelte";
  import Modal from "../Modal.svelte";
  import { svn, relative } from "../svn";
  import { failed, useSession } from "../session.svelte";

  let { path }: { path: string } = $props();
  const s = useSession();
  let to = $state(untrack(() => path));

  async function submit() {
    if (!to || to === path) return;
    s.close();
    if ((await s.run("Moving", () => svn.Move(path, to))) !== failed) s.notice = `Moved to ${relative(s.wc, to)}.`;
    await s.refresh();
  }
</script>

<Modal title="Rename / move" onclose={s.close}>
  <label><span>From</span><input value={path} readonly /></label>
  <label><span>To</span><input bind:value={to} /></label>
  {#snippet footer()}
    <button onclick={s.close}>Cancel</button>
    <button class="primary" disabled={!to || to === path} onclick={submit}>Move</button>
  {/snippet}
</Modal>
