<script lang="ts">
  import { untrack } from "svelte";
  import Modal from "../Modal.svelte";
  import { svn } from "../svn";
  import { failed, baseName, useSession } from "../session.svelte";

  let { paths }: { paths: string[] } = $props();
  const s = useSession();
  // Kept from when the window opened: the props come from the open dialog, which is gone once the window closes, and
  // the work below goes on after that.
  const files = untrack(() => paths);
  let message = $state("");
  let steal = $state(false);
  const one = files.length === 1;

  async function submit() {
    s.close();
    if ((await s.run("Getting the lock", () => svn.Lock(files, message, steal), s.info?.repositoryRoot)) !== failed) {
      s.notice = `Locked ${one ? baseName(files[0]) : `${files.length} files`}. Others cannot commit ${one ? "it" : "them"} until you release the lock or commit.`;
    }
    await s.refresh();
  }
</script>

<Modal title="Get lock" onclose={s.close}>
  <p class="muted">A lock tells everyone you are editing {one ? baseName(files[0]) : `these ${files.length} files`}; nobody else can commit {one ? "it" : "them"} until you commit or release it. Meant for files that cannot be merged, such as images.</p>
  <!-- svelte-ignore a11y_autofocus -->
  <label><span>Why (optional)</span><input bind:value={message} placeholder="Editing the level layout" autofocus /></label>
  <label class="check"><input type="checkbox" bind:checked={steal} /> Take it over even if someone else holds it</label>
  {#snippet footer()}
    <button onclick={s.close}>Cancel</button>
    <button class="primary" disabled={!!s.busy} onclick={submit}>Get lock</button>
  {/snippet}
</Modal>
