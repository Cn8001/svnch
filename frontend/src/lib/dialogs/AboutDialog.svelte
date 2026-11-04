<script lang="ts">
  import { onMount } from "svelte";
  import { Browser } from "@wailsio/runtime";
  import Modal from "../Modal.svelte";
  import { AppService, type AppInfo } from "../svn";
  import { useSession } from "../session.svelte";
  import { REPO_URL } from "../menus";

  const s = useSession();
  let info = $state<AppInfo | null>(null);
  let licences = $state(false);

  onMount(async () => {
    try {
      info = await AppService.About();
    } catch {}
  });
</script>

{#if licences}
  <Modal title="Third-party licences" wide onclose={() => (licences = false)}>
    {#if info?.notices}<pre class="output notices">{info.notices}</pre>{/if}
    <div class="licenses">
      {#each info?.licenses ?? [] as l (l.name + l.version)}
        <details>
          <summary><b>{l.name}</b> <span class="muted">{l.version}</span> <span class="lic">{l.license || "see text"}</span></summary>
          <pre class="output">{l.text}</pre>
          {#if l.notice}<pre class="output">{l.notice}</pre>{/if}
        </details>
      {/each}
    </div>
    {#snippet footer()}
      <button onclick={() => (licences = false)}>Back</button>
    {/snippet}
  </Modal>
{:else}
  <Modal title="About svnch" onclose={s.close}>
    <div class="about">
      <img src="/appicon.png" alt="svnch" />
      <div>
        <h1>svnch {#if info?.version}<span class="ver">{info.version}</span>{/if}</h1>
        <p>A small desktop client for Subversion.<br />It runs the svn commands you would type yourself.</p>
        <p class="muted">{info?.copyright || "(c) 2026 Canercan Demir, MIT License"}</p>
        {#if info}<p class="muted">{info.platform} · {info.go}{info.wails ? ` · Wails ${info.wails}` : ""}</p>{/if}
        <div class="row wrap">
          <button onclick={() => Browser.OpenURL(REPO_URL)}>github.com/Cn8001/svnch</button>
          <button onclick={() => (licences = true)} disabled={!info}>Third-party licences</button>
        </div>
      </div>
    </div>
  </Modal>
{/if}

<style>
  .about { display: flex; gap: 18px; align-items: center; }
  .about img { width: 104px; height: 104px; }
  .about h1 { margin: 0 0 6px; font-size: 20px; }
  .about p { margin: 0 0 10px; }
  .ver { font-weight: normal; color: var(--muted); }
  .notices { max-height: 28vh; }
  .licenses { border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); overflow: hidden; }
  .licenses details { border-bottom: 1px solid var(--line); }
  .licenses details:last-child { border-bottom: 0; }
  .licenses summary { padding: 6px 10px; cursor: pointer; }
  .licenses summary:hover { background: var(--hover); }
  .licenses details[open] summary { background: var(--raised); }
  .licenses details .output { margin: 8px 10px 10px; max-height: 40vh; }
  .lic { float: right; }
</style>
