<script lang="ts">
  import Modal from "../Modal.svelte";
  import { svn } from "../svn";
  import { failed, useSession } from "../session.svelte";

  let { paths }: { paths: string[] } = $props();
  const s = useSession();
  let name = $state("");

  async function move(to: string) {
    const target = to.trim();
    if (!target) return;
    s.close();
    if ((await s.run(`Moving to ${target}`, () => svn.SetChangelist(target, paths))) === failed) return;
    await s.refresh();
  }
</script>

<Modal title="Move to changelist" onclose={s.close}>
  <p class="muted">{paths.length} {paths.length === 1 ? "file" : "files"}. A changelist groups changes so you can commit them on their own; svn forgets it once they are committed.</p>
  {#if s.changelists.length}
    <div class="row wrap">
      {#each s.changelists as cl}<button onclick={() => move(cl)}>{cl}</button>{/each}
    </div>
  {/if}
  <form class="row" onsubmit={(e) => { e.preventDefault(); move(name); }}>
    <!-- svelte-ignore a11y_autofocus -->
    <input bind:value={name} placeholder="New changelist name" autofocus />
    <button class="primary" type="submit" disabled={!name.trim() || name.trim().startsWith("-")}>Move</button>
  </form>
</Modal>
