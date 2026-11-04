<script lang="ts">
  import Modal from "../Modal.svelte";
  import { useSession } from "../session.svelte";

  const s = useSession();
</script>

{#if s.cert.open && s.cert.info}
  {@const c = s.cert.info}
  <Modal title="Untrusted certificate" onclose={() => s.finishTrust(false)}>
    <p class="certnote">
      <b>{c.realm.replace("https://", "")}</b> sent a certificate this computer does not trust, because
      {c.problems?.join(" and ")}.
    </p>
    <p class="muted fine">
      That is normal for a server inside a company network with its own certificate. Trust it if the fingerprint
      matches the one your server admin sees (VisualSVN Server Manager shows it under the server's certificate).
    </p>
    <dl class="kv">
      <dt>Issued to</dt>
      <dd>{c.subject}</dd>
      <dt>Issued by</dt>
      <dd>{c.issuer}</dd>
      {#if c.names?.length}
        <dt>Names</dt>
        <dd>{c.names.join(", ")}</dd>
      {/if}
      <dt>Valid</dt>
      <dd>{new Date(c.validFrom).toLocaleDateString()} to {new Date(c.validUntil).toLocaleDateString()}</dd>
      <dt>Fingerprint</dt>
      <dd class="mono">{c.fingerprint}</dd>
    </dl>
    {#if s.cert.error}<p class="error">{s.cert.error}</p>{/if}
    <p class="muted fine">svnch saves it where svn keeps the certificates you accept, so TortoiseSVN trusts it too.</p>
    {#snippet footer()}
      <button onclick={() => s.finishTrust(false)}>Cancel</button>
      <button class="primary" disabled={s.cert.busy} onclick={s.submitTrust}>{s.cert.busy ? "Saving..." : "Trust this certificate"}</button>
    {/snippet}
  </Modal>
{/if}

<style>
  .certnote { margin: 0; }
</style>
