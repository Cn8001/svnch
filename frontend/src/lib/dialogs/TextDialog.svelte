<script lang="ts">
  import Modal from "../Modal.svelte";
  import { useSession } from "../session.svelte";

  let { title, text }: { title: string; text: string } = $props();
  const s = useSession();
</script>

<Modal {title} wide onclose={s.close}>
  <pre class="output">{text}</pre>
</Modal>
