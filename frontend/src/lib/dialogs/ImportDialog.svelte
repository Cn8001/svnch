<script lang="ts">
  import { untrack } from "svelte";
  import Modal from "../Modal.svelte";
  import { svn } from "../svn";
  import { failed, baseName, useSession } from "../session.svelte";

  let { path }: { path: string } = $props();
  const s = useSession();
  let url = $state("");
  let message = $state(untrack(() => `Import ${baseName(path)}`));

  async function submit() {
    const into = url.trim();
    s.close();
    const rev = await s.run(`Importing ${baseName(path)}`, () => svn.Import(path, into, message.trim()), into);
    if (rev !== failed) s.notice = `Imported into ${into} as r${rev}. The folder itself is not a working copy; check it out to work on it.`;
  }
</script>

<Modal title="Import" onclose={s.close}>
  <p class="muted">Adds the contents of <code>{path}</code> to the repository in one commit. The folder stays as it is, not a working copy.</p>
  <!-- svelte-ignore a11y_autofocus -->
  <label><span>Into (repository url)</span><input bind:value={url} list="import-servers" class="mono" placeholder="https://svn.example.org/svn/Repo/trunk/new" autofocus /></label>
  <datalist id="import-servers">{#each s.servers as srv (srv.url)}<option value={srv.url}>{srv.name}</option>{/each}</datalist>
  <label><span>Commit message</span><input bind:value={message} /></label>
  {#snippet footer()}
    <button onclick={s.close}>Cancel</button>
    <button class="primary" disabled={!url.trim() || !message.trim() || !!s.busy} onclick={submit}>Import</button>
  {/snippet}
</Modal>
