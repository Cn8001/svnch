<script lang="ts">
  import { untrack } from "svelte";
  import Modal from "../Modal.svelte";
  import { svn, Depth, errorText, join, type CheckoutRequest } from "../svn";
  import { failed, useSession } from "../session.svelte";
  import { useActions } from "../actions";

  let { url = "", path = "" }: { url?: string; path?: string } = $props();
  const s = useSession();
  const a = useActions();

  // Where it starts; the fields are the person's from then on.
  let co = $state(untrack(() => ({ url, path, revision: "", depth: Depth.DepthDefault })));

  async function browse() {
    const dir = await a.pickFolder("Check out into", "Choose", true);
    if (!dir) return;
    const name = co.url.replace(/\/+$/, "").split("/").pop() ?? "";
    co.path = name ? join(dir, name) : dir;
  }

  async function submit() {
    const rev = Number(co.revision || 0);
    if (!co.url || !co.path || !Number.isInteger(rev) || rev < 0) return;
    s.close();
    const req: CheckoutRequest = { url: co.url.trim(), path: co.path.trim(), revision: rev, depth: co.depth };
    // Counting first gives the bar a total, and asks for a login or a certificate before the long part starts.
    const total = await s.run("Counting files", () => svn.CountFiles(req.url), req.url);
    const r = await s.run(`Checking out ${req.url}`, () => svn.Checkout(req), req.url, total === failed ? 0 : total);
    if (r === failed) {
      // svn can fail at the very end (an external, a warning it counts as an error) after the working copy is
      // complete; open what is there, and leave the error in the status bar.
      const why = s.errorFull;
      try {
        await svn.Info(req.path);
      } catch {
        return;
      }
      await s.openWC(req.path);
      s.error = errorText(why);
      s.errorFull = why;
      return;
    }
    await s.openWC(req.path);
    s.notice = `Checked out revision ${r}.`;
  }
</script>

<Modal title="Checkout" onclose={s.close}>
  <label>
    <span>Repository URL</span>
    <div class="row">
      <input bind:value={co.url} list="saved-servers" placeholder="https://svn.example.org/repo/trunk" />
      <button type="button" onclick={() => a.openRepos()}>Browse...</button>
    </div>
  </label>
  <datalist id="saved-servers">
    {#each s.servers as srv (srv.url)}<option value={srv.url}>{srv.name}</option>{/each}
  </datalist>
  <label>
    <span>Into folder</span>
    <div class="row"><input bind:value={co.path} placeholder="/home/me/work/trunk" /><button type="button" onclick={browse}>Browse...</button></div>
  </label>
  <div class="row">
    <label class="grow"><span>Revision (empty for HEAD)</span><input bind:value={co.revision} inputmode="numeric" /></label>
    <label class="grow">
      <span>Depth</span>
      <select bind:value={co.depth}>
        <option value={Depth.DepthDefault}>everything</option>
        <option value={Depth.DepthImmediates}>immediates</option>
        <option value={Depth.DepthFiles}>files only</option>
        <option value={Depth.DepthEmpty}>empty</option>
      </select>
    </label>
  </div>
  {#snippet footer()}
    <button onclick={s.close}>Cancel</button>
    <button class="primary" disabled={!co.url || !co.path} onclick={submit}>Checkout</button>
  {/snippet}
</Modal>
