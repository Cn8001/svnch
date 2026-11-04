<script lang="ts">
  import Modal from "../Modal.svelte";
  import { svn } from "../svn";
  import { failed, useSession } from "../session.svelte";

  const s = useSession();
  let name = $state("");
  let message = $state("");

  async function submit() {
    const n = name.trim();
    if (!n) return;
    s.close();
    if ((await s.run("Shelving", () => svn.Shelve(s.wc, n, message))) !== failed) s.notice = `Shelved as ${n}.`;
    await s.refresh();
  }
</script>

<Modal title="Shelve changes" onclose={s.close}>
  <p class="muted">Puts every local change aside and reverts the working copy. Needs svn 1.14 or newer.</p>
  <label><span>Name</span><input bind:value={name} placeholder="half-done-feature" /></label>
  <label><span>Note (optional)</span><input bind:value={message} /></label>
  {#snippet footer()}
    <button onclick={s.close}>Cancel</button>
    <button class="primary" disabled={!name.trim()} onclick={submit}>Shelve</button>
  {/snippet}
</Modal>
