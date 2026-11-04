<script lang="ts">
  import Modal from "../Modal.svelte";
  import SvnMissing from "../SvnMissing.svelte";
  import { useSession } from "../session.svelte";

  const s = useSession();
</script>

<Modal title="Installing svn" onclose={s.close}>
  {#if s.setup?.binary}
    <p>svn {s.setup.version} is installed at <code>{s.setup.binary}</code>.</p>
  {:else}
    <SvnMissing setup={s.setup} checking={s.checkingSvn} oncheck={s.checkSvn} />
  {/if}
</Modal>
