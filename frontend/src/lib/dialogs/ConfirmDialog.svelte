<script lang="ts">
  import Modal from "../Modal.svelte";
  import { useSession } from "../session.svelte";

  let { title, text, ok, run }: { title: string; text: string; ok: string; run: () => void } = $props();
  const s = useSession();
</script>

<Modal {title} onclose={s.close}>
  <pre class="note">{text}</pre>
  {#snippet footer()}
    <button onclick={s.close}>Cancel</button>
    <button class="primary" onclick={() => { s.close(); run(); }}>{ok}</button>
  {/snippet}
</Modal>
