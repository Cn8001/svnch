<script lang="ts">
  import { onMount } from "svelte";
  import Modal from "../Modal.svelte";
  import { svn, ExplorerService, UpdateService, errorText, type ExplorerStatus } from "../svn";
  import { useSession } from "../session.svelte";
  import { useTheme, type ThemeName } from "../theme.svelte";

  const s = useSession();
  const theme = useTheme();
  let note = $state("");
  let explorer = $state<ExplorerStatus | null>(null);
  const themes: [ThemeName, string][] = [["dark", "Dark"], ["light", "Light"], ["system", "System"]];

  onMount(async () => {
    try {
      s.sessionUser = await svn.LoggedInAs();
    } catch {}
    try {
      explorer = await ExplorerService.Status();
    } catch {}
  });

  // Check only starts the updater; Wails opens its own update window with the progress and the result.
  async function checkUpdates() {
    note = "Checking...";
    try {
      await UpdateService.Check();
      note = "The update window shows the result.";
    } catch (e) {
      note = errorText(e);
    }
  }

  async function forgetLogin() {
    await svn.Logout();
    s.sessionUser = "";
    note = "Forgotten for this session. svn may still have the password saved in its own store.";
  }

  async function toggleExplorer() {
    if (!explorer) return;
    try {
      await ExplorerService.SetEnabled(!explorer.enabled);
      explorer = await ExplorerService.Status();
    } catch (e) {
      note = errorText(e);
    }
  }
</script>

<Modal title="Settings" onclose={s.close}>
  <div class="setting">
    <div>
      <b>Theme</b>
      <p class="muted">System follows your desktop's light or dark setting.</p>
    </div>
    <div class="segmented" role="group" aria-label="Theme">
      {#each themes as [value, label]}
        <button class:on={theme.name === value} onclick={() => theme.set(value)}>{label}</button>
      {/each}
    </div>
  </div>
  <div class="setting">
    <div>
      <b>Updates</b>
      <p class="muted">svnch looks for a new release each time it starts.</p>
    </div>
    <button onclick={checkUpdates}>Check for updates</button>
  </div>
  <div class="setting">
    <div>
      <b>Login</b>
      <p class="muted">{s.sessionUser ? `Logged in as ${s.sessionUser} for this session.` : "Not logged in this session; svn may still use a password it saved."}</p>
    </div>
    <div class="buttons">
      <button onclick={() => s.startLogin()}>{s.sessionUser ? "Switch user..." : "Log in..."}</button>
      <button onclick={forgetLogin}>Forget login</button>
    </div>
  </div>
  {#if explorer?.supported}
    <div class="setting">
      <div>
        <b>Explorer</b>
        <p class="muted">An svnch menu with TortoiseSVN's commands, update and commit first, when you right-click a folder or file. On Windows 11 it is under "Show more options".</p>
      </div>
      <button onclick={toggleExplorer}>{explorer.enabled ? "Remove from menu" : "Add to menu"}</button>
    </div>
  {/if}
  {#if note}<p>{note}</p>{/if}
</Modal>

<style>
  .setting { display: flex; justify-content: space-between; align-items: center; gap: 16px; }
  .setting p { margin: 4px 0 0; }
  .setting .buttons { display: flex; gap: 6px; }
  .setting + .setting { margin-top: 2px; padding-top: 12px; border-top: 1px solid var(--line); }
  .segmented { display: flex; padding: 2px; gap: 2px; border: 1px solid var(--line-strong); border-radius: var(--radius); background: var(--field); }
  .segmented button { border: 0; padding: 3px 10px; background: transparent; color: var(--muted); }
  .segmented button:hover:not(:disabled) { background: var(--hover); color: var(--text); }
  .segmented button.on { background: var(--raised); color: var(--text); }
</style>
