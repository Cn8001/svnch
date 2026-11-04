<script lang="ts">
  import Modal from "../Modal.svelte";
  import { useSession } from "../session.svelte";

  const s = useSession();
</script>

{#if s.login.open}
  <Modal title="Login" onclose={() => s.finishLogin(false)}>
    <form class="login" onsubmit={(e) => { e.preventDefault(); s.submitLogin(); }}>
      {#if s.login.manual}
        <p>Log in to a server for this session.</p>
        <label><span>Server</span><input bind:value={s.login.url} list="login-servers" placeholder="https://svn.example.org/svn" /></label>
        <datalist id="login-servers">
          {#each s.servers as srv (srv.url)}<option value={srv.url}>{srv.name}</option>{/each}
        </datalist>
      {:else}
        <p>The server asks who you are.</p>
        <p class="muted where" title={s.login.url}>{s.login.url}</p>
      {/if}
      <!-- svelte-ignore a11y_autofocus -->
      <label><span>Username</span><input bind:value={s.login.username} autofocus={!s.login.username} autocomplete="username" /></label>
      <!-- svelte-ignore a11y_autofocus -->
      <label><span>Password</span><input type="password" bind:value={s.login.password} autofocus={!!s.login.username} autocomplete="current-password" /></label>
      {#if s.login.error}<p class="error">{s.login.error}</p>{/if}
      <p class="muted fine">svn saves the password in your system keyring when it can. Where it cannot, svnch keeps it until you close the app.</p>
      <div class="end">
        <button type="button" onclick={() => s.finishLogin(false)}>Cancel</button>
        <button class="primary" type="submit" disabled={!s.login.username || !s.login.url.trim() || s.login.busy}>{s.login.busy ? "Checking..." : "Log in"}</button>
      </div>
    </form>
  </Modal>
{/if}

<style>
  .login { display: grid; gap: 12px; }
  .login p { margin: 0; }
  .where { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: var(--mono); font-size: 12px; }
</style>
