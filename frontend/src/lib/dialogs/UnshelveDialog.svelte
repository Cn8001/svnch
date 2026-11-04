<script lang="ts">
  import { onMount } from "svelte";
  import Modal from "../Modal.svelte";
  import { svn } from "../svn";
  import { failed, useSession } from "../session.svelte";

  const s = useSession();
  let shelves = $state<string[] | null>(null);

  onMount(async () => {
    const r = await s.run("Reading shelves", () => svn.Shelves(s.wc));
    shelves = r === failed ? [] : (r ?? []);
  });

  async function unshelve(name: string) {
    s.close();
    if ((await s.run("Unshelving", () => svn.Unshelve(s.wc, name))) !== failed) s.notice = `Unshelved ${name}.`;
    await s.refresh();
  }
</script>

<Modal title="Unshelve" onclose={s.close}>
  {#if shelves === null}
    <p class="muted">Reading shelves...</p>
  {:else}
    {#each shelves as name}
      <div class="row between"><span>{name}</span><button onclick={() => unshelve(name)}>Unshelve</button></div>
    {:else}
      <p class="muted">No shelves in this working copy.</p>
    {/each}
  {/if}
</Modal>
