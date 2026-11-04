<script lang="ts">
  import { Browser, Clipboard } from "@wailsio/runtime";
  import type { Setup } from "./svn";

  let { setup, checking = false, oncheck }: { setup: Setup | null; checking?: boolean; oncheck: () => void } = $props();

  const ways: Record<string, { label: string; command: string }[]> = {
    linux: [
      { label: "Debian, Ubuntu", command: "sudo apt install subversion" },
      { label: "Fedora", command: "sudo dnf install subversion" },
      { label: "Arch", command: "sudo pacman -S subversion" },
      { label: "openSUSE", command: "sudo zypper install subversion" },
    ],
    darwin: [{ label: "Homebrew", command: "brew install subversion" }],
    windows: [{ label: "winget", command: "winget install Slik.Subversion" }],
  };

  let os = $derived(setup?.os ?? "linux");
  let copied = $state("");

  async function copy(command: string) {
    await Clipboard.SetText(command);
    copied = command;
    setTimeout(() => copied === command && (copied = ""), 1500);
  }
</script>

<div class="missing">
  <h2>svn is not installed</h2>
  <p>svnch runs the <code>svn</code> command line tool and could not find it on this computer.</p>
  {#if setup?.problem}<p class="muted small">Automatic install: {setup.problem}</p>{/if}

  <p>Install it from a terminal:</p>
  <ul>
    {#each ways[os] ?? ways.linux as w}
      <li>
        <span class="muted">{w.label}</span>
        <code>{w.command}</code>
        <button class="small" onclick={() => copy(w.command)}>{copied === w.command ? "Copied" : "Copy"}</button>
      </li>
    {/each}
  </ul>
  {#if os === "windows"}
    <p class="muted small">TortoiseSVN works too, if you tick "command line client tools" in its installer.</p>
  {:else if os === "darwin"}
    <p class="muted small">No Homebrew yet? It is one line from brew.sh.</p>
  {/if}

  <div class="actions">
    <button class="primary" onclick={oncheck} disabled={checking}>{checking ? "Checking..." : "Check again"}</button>
    <button onclick={() => Browser.OpenURL("https://subversion.apache.org/packages.html")}>Other ways to get svn</button>
  </div>
</div>

<style>
  .missing { max-width: 560px; display: grid; gap: 8px; text-align: left; }
  h2 { margin: 0; font-size: 16px; }
  p { margin: 0; }
  ul { margin: 0; padding: 0; list-style: none; display: grid; gap: 6px; }
  li { display: grid; grid-template-columns: 9em 1fr auto; align-items: center; gap: 8px; }
  code { font-weight: bold; }
  li code { font-weight: normal; padding: 3px 6px; background: var(--field); border: 2px solid var(--ink); user-select: text; }
  .small { font-size: 12px; }
  button.small { padding: 1px 8px; box-shadow: 1px 1px 0 var(--ink); }
  .actions { display: flex; gap: 10px; margin-top: 6px; }
</style>
