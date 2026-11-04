<script lang="ts">
  import Modal from "../Modal.svelte";
  import { svn } from "../svn";
  import { failed, baseName, useSession } from "../session.svelte";

  let { paths }: { paths: string[] } = $props();
  const s = useSession();
  let message = $state("");
  let steal = $state(false);
  let one = $derived(paths.length === 1);

  async function submit() {
    s.close();
    if ((await s.run("Getting the lock", () => svn.Lock(paths, message, steal), s.info?.repositoryRoot)) !== failed) {
      s.notice = `Locked ${one ? baseName(paths[0]) : `${paths.length} files`}. Others cannot commit ${one ? "it" : "them"} until you release the lock or commit.`;
    }
    await s.refresh();
  }
</script>

<Modal title="Get lock" onclose={s.close}>
  <p class="muted">A lock tells everyone you are editing {one ? baseName(paths[0]) : `these ${paths.length} files`}; nobody else can commit {one ? "it" : "them"} until you commit or release it. Meant for files that cannot be merged, such as images.</p>
  <!-- svelte-ignore a11y_autofocus -->
  <label><span>Why (optional)</span><input bind:value={message} placeholder="Editing the level layout" autofocus /></label>
  <label class="check"><input type="checkbox" bind:checked={steal} /> Take it over even if someone else holds it</label>
  {#snippet footer()}
    <button onclick={s.close}>Cancel</button>
    <button class="primary" disabled={!!s.busy} onclick={submit}>Get lock</button>
  {/snippet}
</Modal>
