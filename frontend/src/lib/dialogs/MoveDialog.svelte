<script lang="ts">
  import { untrack } from "svelte";
  import Modal from "../Modal.svelte";
  import { svn, relative } from "../svn";
  import { failed, useSession } from "../session.svelte";

  let { path }: { path: string } = $props();
  const s = useSession();
  // Kept from when the window opened: the props come from the open dialog, which is gone once the window closes, and
  // the work below goes on after that.
  const from = untrack(() => path);
  let to = $state(from);

  async function submit() {
    if (!to || to === from) return;
    s.close();
    if ((await s.run("Moving", () => svn.Move(from, to))) !== failed) s.notice = `Moved to ${relative(s.wc, to)}.`;
    await s.refresh();
  }
</script>

<Modal title="Rename / move" onclose={s.close}>
  <label><span>From</span><input value={from} readonly /></label>
  <label><span>To</span><input bind:value={to} /></label>
  {#snippet footer()}
    <button onclick={s.close}>Cancel</button>
    <button class="primary" disabled={!to || to === from} onclick={submit}>Move</button>
  {/snippet}
</Modal>
