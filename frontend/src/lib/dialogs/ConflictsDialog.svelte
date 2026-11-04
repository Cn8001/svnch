<script lang="ts">
  import Modal from "../Modal.svelte";
  import ConflictPanel from "../ConflictPanel.svelte";
  import { useSession } from "../session.svelte";
  import { useActions } from "../actions";

  let { first = "" }: { first?: string } = $props();
  const s = useSession();
  const a = useActions();
</script>

<Modal title="Resolve conflicts" wide onclose={s.close}>
  {#if s.conflicts.length}
    <ConflictPanel wc={s.wc} conflicts={s.conflicts} {first} resolve={a.resolveConflicts} openFile={a.openLocal} />
  {:else}
    <p>No conflicts left.</p>
  {/if}
</Modal>
