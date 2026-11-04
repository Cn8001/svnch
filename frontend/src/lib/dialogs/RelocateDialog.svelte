<script lang="ts">
  import Modal from "../Modal.svelte";
  import { svn } from "../svn";
  import { failed, useSession } from "../session.svelte";

  const s = useSession();
  let to = $state(s.info?.url ?? "");

  async function submit() {
    const target = to.trim();
    s.close();
    if ((await s.run("Relocating", () => svn.Relocate(s.wc, target), target)) === failed) return;
    await s.reloadInfo();
    s.notice = `This working copy now points at ${target}.`;
  }
</script>

<Modal title="Relocate" onclose={s.close}>
  <p class="muted">For when the server moved, for example to a new address or https: points this working copy at the same repository under its new url. svn checks that it really is the same repository.</p>
  <label><span>New url of this working copy</span><input bind:value={to} class="mono" /></label>
  {#snippet footer()}
    <button onclick={s.close}>Cancel</button>
    <button class="primary" disabled={!to.trim() || to.trim() === s.info?.url || !!s.busy} onclick={submit}>Relocate</button>
  {/snippet}
</Modal>
