<script lang="ts">
  import { untrack } from "svelte";
  import { relative, svn, type FileContent, type StatusEntry } from "./svn";

  let {
    wc,
    conflicts,
    first = "",
    resolve,
    openFile,
  }: {
    wc: string;
    conflicts: StatusEntry[];
    first?: string; // the file to show when the panel opens
    resolve: (accept: string, paths: string[]) => Promise<boolean>;
    openFile: (path: string) => void;
  } = $props();

  // Only where the panel opens; after that the list decides.
  let selected = $state(untrack(() => first));
  let content = $state<FileContent | null>(null);
  let readError = $state("");
  let working = $state(false);

  let current = $derived(conflicts.find((c) => c.path === selected) ?? conflicts[0]);
  let textual = $derived(conflicts.filter((c) => !c.treeConflicted));
  let trees = $derived(conflicts.length - textual.length);

  function kind(c: StatusEntry): string {
    if (c.treeConflicted) return "tree";
    if (c.item === "conflicted") return "text";
    return "properties";
  }

  // Each line gets the side it belongs to between svn's markers, so mine and theirs read apart at a glance.
  function marked(text: string): { line: string; part: string }[] {
    let part = "";
    return text.split("\n").map((line) => {
      if (line.startsWith("<<<<<<<")) {
        part = "mine";
        return { line, part: "marker" };
      }
      if (line.startsWith("|||||||")) {
        part = "base";
        return { line, part: "marker" };
      }
      if (line.startsWith("=======") && part) {
        part = "theirs";
        return { line, part: "marker" };
      }
      if (line.startsWith(">>>>>>>")) {
        part = "";
        return { line, part: "marker" };
      }
      return { line, part };
    });
  }

  $effect(() => {
    const c = current;
    content = null;
    readError = "";
    if (!c || c.treeConflicted) return;
    svn.ReadLocal(wc, c.path).then(
      (r) => current === c && (content = r),
      (e) => current === c && (readError = e instanceof Error ? e.message : String(e)),
    );
  });

  async function settle(accept: string, paths: string[]) {
    if (!paths.length || working) return;
    working = true;
    await resolve(accept, paths);
    working = false;
  }
</script>

<div class="conflicts">
  <div class="all">
    <span>
      <b>{conflicts.length}</b> {conflicts.length === 1 ? "file conflicts" : "files conflict"}.
      <span class="muted">Mine is your local version; theirs came from the server.</span>
    </span>
    <span class="grow"></span>
    <button disabled={!textual.length || working} onclick={() => settle("mine-full", textual.map((c) => c.path))}>Accept mine for all</button>
    <button disabled={!textual.length || working} onclick={() => settle("theirs-full", textual.map((c) => c.path))}>Accept theirs for all</button>
  </div>
  {#if trees}<p class="muted fine">{trees} tree {trees === 1 ? "conflict needs" : "conflicts need"} a look of its own and {trees === 1 ? "is" : "are"} left out of the buttons above.</p>{/if}

  <div class="split">
    <ul class="files">
      {#each conflicts as c (c.path)}
        <li>
          <button class:sel={current?.path === c.path} onclick={() => (selected = c.path)} title={c.path}>
            <span class="name">{relative(wc, c.path)}</span>
            <span class="kind">{kind(c)}</span>
          </button>
        </li>
      {/each}
    </ul>

    {#if current}
      <div class="detail">
        <div class="actions">
          {#if current.treeConflicted}
            <button class="primary" disabled={working} onclick={() => settle("working", [current.path])}>Mark resolved</button>
            <button onclick={() => openFile(current.path)}>Open</button>
          {:else}
            <button disabled={working} onclick={() => settle("mine-full", [current.path])}>Use mine</button>
            <button disabled={working} onclick={() => settle("theirs-full", [current.path])}>Use theirs</button>
            {#if current.item === "conflicted"}
              <button disabled={working} onclick={() => settle("mine-conflict", [current.path])} title="Keep their changes that do not clash, and mine where they do">Mine where they clash</button>
              <button disabled={working} onclick={() => settle("theirs-conflict", [current.path])} title="Keep my changes that do not clash, and theirs where they do">Theirs where they clash</button>
            {/if}
            <span class="grow"></span>
            <button onclick={() => openFile(current.path)}>Open in editor</button>
            <button class="primary" disabled={working} onclick={() => settle("working", [current.path])} title="Use the file as it is now, after editing it by hand">Mark resolved</button>
          {/if}
        </div>
        {#if current.treeConflicted}
          <p class="note">
            A tree conflict: the same file or folder was moved, deleted or added both here and on the server. svn cannot
            pick a side for these by itself. Put the files the way they should be, then press Mark resolved.
          </p>
        {:else if readError}
          <p class="bad note">{readError}</p>
        {:else if !content}
          <p class="muted note">Reading...</p>
        {:else if content.binary}
          <p class="note">This file is not text, so there is nothing to merge line by line. Use mine or use theirs.</p>
        {:else if current.item !== "conflicted"}
          <p class="note">Only the file's svn properties conflict, not its content. Use mine or use theirs.</p>
        {:else}
          <pre class="text">{#each marked(content.text) as l}<span class="l {l.part}">{l.line || " "}</span>{/each}</pre>
        {/if}
      </div>
    {/if}
  </div>
</div>

<style>
  .conflicts { display: grid; grid-template-rows: auto auto minmax(0, 1fr); gap: 10px; height: 64vh; }
  .all { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .grow { flex: 1; }
  .fine { margin: -4px 0 0; font-size: 12px; }
  .split { display: grid; grid-template-columns: 240px minmax(0, 1fr); gap: 12px; min-height: 0; }
  .files { margin: 0; padding: 4px; list-style: none; overflow: auto; border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); }
  .files button { display: flex; justify-content: space-between; gap: 8px; width: 100%; border: 0; border-radius: 4px; background: transparent; text-align: left; padding: 5px 8px; }
  .files button:hover:not(:disabled) { background: var(--hover); }
  .files button.sel { background: var(--accent-soft); box-shadow: inset 2px 0 0 var(--accent); }
  .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--danger); }
  .kind { font-size: 11px; color: var(--muted); }
  .detail { display: grid; grid-template-rows: auto minmax(0, 1fr); gap: 8px; min-height: 0; min-width: 0; }
  .actions { display: flex; gap: 6px; flex-wrap: wrap; align-items: center; }
  .note { margin: 0; }
  .bad { color: var(--danger); }
  .text { margin: 0; overflow: auto; border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); padding: 6px 0; line-height: 1.5; }
  .l { display: block; padding: 0 10px; white-space: pre; }
  .l.mine { background: var(--add-bg); color: var(--add-ink); }
  .l.theirs { background: var(--hunk-bg); color: var(--hunk-ink); }
  .l.base { color: var(--muted); }
  .l.marker { color: var(--accent); font-weight: 700; background: var(--accent-soft); }
</style>
