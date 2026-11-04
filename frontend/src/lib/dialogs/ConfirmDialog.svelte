<script lang="ts">
  import { untrack } from "svelte";
  import Modal from "../Modal.svelte";
  import { useSession } from "../session.svelte";

  let { title, text, ok, run }: { title: string; text: string; ok: string; run: () => void } = $props();
  const s = useSession();
  // Kept from when the window opened: the props come from the open dialog, which is gone once the window closes, and
  // the work below goes on after that.
  const action = untrack(() => run);
</script>

<Modal {title} onclose={s.close}>
  <pre class="note">{text}</pre>
  {#snippet footer()}
    <button onclick={s.close}>Cancel</button>
    <button class="primary" onclick={() => { s.close(); action(); }}>{ok}</button>
  {/snippet}
</Modal>
