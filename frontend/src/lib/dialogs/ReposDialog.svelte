<script lang="ts">
  import Modal from "../Modal.svelte";
  import RepoBrowser from "../RepoBrowser.svelte";
  import { svn, ServerService, errorText, type Server } from "../svn";
  import { failed, useSession } from "../session.svelte";
  import { useActions } from "../actions";

  let { at = "" }: { at?: string } = $props();
  const s = useSession();
  const a = useActions();

  async function list(url: string) {
    const r = await s.run(`Listing ${url}`, () => svn.List(url), url);
    if (r !== failed) return r ?? [];
    return s.lastFailure === "TIMEOUT" ? "No answer within a minute." : s.error || "Could not list it.";
  }

  // The check goes through run, so a server that wants a password brings up the login with the form's username.
  async function check(url: string, username: string) {
    const target = url.trim().replace(/\/+$/, "");
    if (username) s.login.username = username;
    const r = await s.run(`Connecting to ${target}`, () => svn.CheckConnection(target), target);
    if (r === failed) {
      const hint = target.startsWith("http://") ? " The server may only answer on https://." : " Check the address, or whether you need a VPN.";
      const text = s.lastFailure === "TIMEOUT" ? `No answer in 10 seconds.${hint}` : s.error || "Could not connect.";
      return { ok: false, text, username: "" };
    }
    const n = (r ?? []).length;
    return { ok: true, text: `Connected. ${n} ${n === 1 ? "entry" : "entries"} at this address.`, username: s.login.url.startsWith(target) ? s.login.username : "" };
  }

  async function save(srv: Server) {
    try {
      await ServerService.Save(srv);
      await s.loadServers();
      return true;
    } catch (e) {
      s.error = errorText(e);
      return false;
    }
  }

  async function remove(url: string) {
    try {
      await ServerService.Remove(url);
    } catch (e) {
      s.error = errorText(e);
    }
    await s.loadServers();
  }
</script>

<Modal title="Repositories" wide onclose={s.close}>
  <RepoBrowser
    servers={s.servers}
    {list}
    onsave={save}
    onremove={remove}
    oncheckout={(url) => a.openCheckout(url)}
    onlog={(url) => a.openLog(url)}
    {check}
    start={at}
    onlogin={s.startLogin}
    exec={s.remote}
    known={s.info ? [s.info.repositoryRoot] : []}
    cancel={() => s.cancelBusy?.()}
  />
</Modal>
