<script lang="ts">
  import { onMount, untrack } from "svelte";
  import Modal from "../Modal.svelte";
  import { svn } from "../svn";
  import { failed, useSession } from "../session.svelte";

  let { topic = "" }: { topic?: string } = $props();
  const s = useSession();
  let asked = $state(untrack(() => topic));
  let text = $state("");

  async function show(t: string) {
    const r = await s.run("Reading help", () => svn.Help(t));
    if (r !== failed) text = r;
  }

  onMount(() => show(asked));
</script>

<Modal title="svn help" wide onclose={s.close}>
  <form class="row" onsubmit={(e) => { e.preventDefault(); show(asked.trim()); }}>
    <input class="grow" bind:value={asked} placeholder="command, e.g. merge" />
    <button type="submit">Show</button>
  </form>
  <pre class="output">{text}</pre>
</Modal>
