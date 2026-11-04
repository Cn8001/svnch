<script lang="ts">
  import Modal from "../Modal.svelte";
  import PropertiesPanel from "../PropertiesPanel.svelte";
  import { useSession } from "../session.svelte";

  let { target, label, isDir }: { target: string; label: string; isDir: boolean } = $props();
  const s = useSession();
</script>

<Modal title="Properties: {label}" wide onclose={s.close}>
  <PropertiesPanel {target} {label} {isDir} remote={false} exec={s.remote} onchanged={s.refresh} />
</Modal>
