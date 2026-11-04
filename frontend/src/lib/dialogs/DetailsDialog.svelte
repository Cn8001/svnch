<script lang="ts">
  import { Clipboard } from "@wailsio/runtime";
  import Modal from "../Modal.svelte";
  import { useSession } from "../session.svelte";
  import { useActions } from "../actions";

  const s = useSession();
  const a = useActions();

  function host(url: string): string {
    try {
      return new URL(url).host || url;
    } catch {
      return url;
    }
  }
</script>

{#if s.info}
  {@const info = s.info}
  <Modal title="Working copy" onclose={s.close}>
    <dl class="kv">
      <dt>Server</dt>
      <dd>{host(info.repositoryRoot)}{#if s.wcServer}<span class="muted"> &middot; saved as {s.wcServer.name}</span>{/if}</dd>
      <dt>Repository</dt>
      <dd class="mono">{info.repositoryRoot}</dd>
      <dt>Branch path</dt>
      <dd class="mono">{info.relativeUrl || "^/"}</dd>
      <dt>URL</dt>
      <dd class="mono">{info.url}</dd>
      <dt>Working copy</dt>
      <dd class="mono">{info.workingCopyRoot || s.wc}</dd>
      <dt>Revision</dt>
      <dd>r{info.revision}</dd>
      <dt>Last change</dt>
      <dd>{[`r${info.lastCommit.revision}`, info.lastCommit.author && `by ${info.lastCommit.author}`, info.lastCommit.date && new Date(info.lastCommit.date).toLocaleString()].filter(Boolean).join(" ")}</dd>
      <dt>Depth</dt>
      <dd>{info.depth || "infinity"}</dd>
      <dt>Logged in as</dt>
      <dd>{s.sessionUser || (s.wcServer?.username ? `${s.wcServer.username} (saved, svn keeps the password)` : "svn's own saved login, if any")}</dd>
      <dt>UUID</dt>
      <dd class="mono muted">{info.repositoryUuid}</dd>
    </dl>
    {#snippet footer()}
      <button onclick={() => Clipboard.SetText(info.url)}>Copy URL</button>
      <button onclick={() => a.openRepos(info.url)}>Browse on server...</button>
      <button class="primary" onclick={s.close}>Close</button>
    {/snippet}
  </Modal>
{/if}
