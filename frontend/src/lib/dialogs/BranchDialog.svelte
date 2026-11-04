<script lang="ts">
  import Modal from "../Modal.svelte";
  import { svn } from "../svn";
  import { failed, useSession } from "../session.svelte";

  const s = useSession();
  let name = $state("");
  let message = $state("");
  let switchAfter = $state(true);
  let l = $derived(s.layout());

  function branchURL(n: string): string {
    return l ? `${l.base}/branches/${encodeURIComponent(n.trim())}` : "";
  }

  async function submit() {
    const layout = l;
    const n = name.trim();
    if (!layout || !n) return;
    const to = branchURL(n);
    const msg = message.trim() || `Create branch ${n}`;
    const after = switchAfter;
    s.close();
    const rev = await s.run(`Creating branch ${n}`, () => svn.Branch({ from: layout.from, to, message: msg }), layout.base);
    if (rev === failed) return;
    if (after) {
      if ((await s.run(`Switching to ${n}`, () => svn.Switch(s.wc, to + layout.sub), layout.base)) === failed) return;
      await s.reloadInfo();
      await s.refresh();
    }
    s.notice = `Branch ${n} created in r${rev}${after ? ", and this working copy is on it now" : ""}.`;
  }
</script>

<Modal title="New branch" onclose={s.close}>
  {#if s.atRoot}
    <p>This working copy is the whole repository (<code>^/</code>), and the repository has no <code>trunk</code> and <code>branches</code> folders.</p>
    <p class="muted">
      A branch in svn is a copy of a folder. svn does let you copy the root into <code>^/branches</code>, but every
      such branch holds the whole repository, the branches made before it included, so they nest deeper each time.
      Branches need the usual layout: the files under <code>^/trunk</code>, branches under <code>^/branches</code>.
      Moving the files there is a single commit, but it changes the address for everyone, so agree on it with your
      team first. VisualSVN Server can also create new repositories with this layout.
    </p>
  {:else}
    <p class="muted">Copies {l?.line || s.info?.relativeUrl} on the server as a new branch, in one commit. No files are transferred.</p>
    <!-- svelte-ignore a11y_autofocus -->
    <label><span>Name</span><input bind:value={name} placeholder="feature-login" autofocus /></label>
    {#if name.trim()}<p class="muted fine mono">{branchURL(name)}</p>{/if}
    <label><span>Message</span><input bind:value={message} placeholder={`Create branch ${name.trim() || "..."}`} /></label>
    <label class="check"><input type="checkbox" bind:checked={switchAfter} /> Switch this working copy to it</label>
    {#if !l?.line}<p class="muted fine">This repository has no trunk or branches folder; the branch goes under ^/branches.</p>{/if}
  {/if}
  {#snippet footer()}
    <button onclick={s.close}>{s.atRoot ? "Close" : "Cancel"}</button>
    {#if !s.atRoot}
      <button class="primary" disabled={!name.trim() || /[\\/]/.test(name) || !!s.busy} onclick={submit}>Create branch</button>
    {/if}
  {/snippet}
</Modal>
