<script lang="ts">
  import { onMount } from "svelte";
  import { svn, type Property } from "./svn";

  type Result<T> = { ok: true; value: T } | { ok: false; error: string };

  let {
    target,
    label,
    isDir,
    remote,
    exec,
    onchanged,
  }: {
    target: string; // a working copy path, or a url when remote
    label: string;
    isDir: boolean;
    remote: boolean; // on the server: each change is a commit of its own
    exec: <T>(label: string, make: () => Promise<T>, url: string) => Promise<Result<T>>;
    onchanged?: () => void;
  } = $props();

  // The properties svn and TortoiseSVN give a meaning to, with what each one does; anything else is a free name.
  const known: { name: string; help: string; dir?: boolean; file?: boolean }[] = [
    { name: "svn:ignore", help: "Names or patterns of new files svn should not offer to add, one per line, for this folder.", dir: true },
    { name: "svn:global-ignores", help: "Like svn:ignore, but for this folder and everything below it.", dir: true },
    { name: "svn:externals", help: "Other repository folders pulled in here, one per line: url folder-name (or -r N url folder-name).", dir: true },
    { name: "svn:auto-props", help: "Properties given to new files by pattern, one per line: *.txt = svn:eol-style=native", dir: true },
    { name: "svn:eol-style", help: "Line endings svn writes this file with on each computer.", file: true },
    { name: "svn:keywords", help: "Keywords svn fills in inside the file, such as $Revision$ or $Author$.", file: true },
    { name: "svn:mime-type", help: "The file's type. A non-text type stops svn from merging or diffing it line by line.", file: true },
    { name: "svn:needs-lock", help: "The file is read-only until someone takes its lock; for files that cannot be merged, such as images.", file: true },
    { name: "svn:executable", help: "Marks the file runnable on Linux and macOS.", file: true },
    { name: "bugtraq:url", help: "Issue tracker link, with %BUGID% where the issue number goes.", dir: true },
    { name: "bugtraq:message", help: "Line added to commit messages for the issue, with %BUGID%.", dir: true },
    { name: "bugtraq:logregex", help: "Regular expression that finds issue numbers in commit messages.", dir: true },
    { name: "bugtraq:label", help: "Label of the issue number field in TortoiseSVN's commit window.", dir: true },
    { name: "tsvn:logminsize", help: "Shortest commit message TortoiseSVN accepts.", dir: true },
    { name: "tsvn:logtemplate", help: "Text TortoiseSVN starts a commit message with.", dir: true },
    { name: "tsvn:projectlanguage", help: "Language code for TortoiseSVN's spell check of commit messages.", dir: true },
  ];
  const eolStyles = ["native", "LF", "CRLF", "CR"];
  const keywordNames = ["Date", "Revision", "Author", "HeadURL", "Id", "Header"];
  const flagProps = ["svn:needs-lock", "svn:executable"];

  let items = $state<Property[]>([]);
  let loading = $state(true);
  let failure = $state("");
  let selected = $state("");
  let form = $state<{ name: string; value: string; recursive: boolean; message: string; editing: boolean; error: string; busy: boolean } | null>(null);

  let help = $derived(form ? known.find((k) => k.name === form!.name.trim())?.help ?? "" : "");
  let suggestions = $derived(known.filter((k) => (isDir ? k.dir : k.file) && !items.some((p) => p.name === k.name)));
  let keywords = $derived(form ? form.value.split(/\s+/).filter(Boolean) : []);

  async function load() {
    loading = true;
    const r = await exec("Reading properties", () => svn.Properties(target), target);
    loading = false;
    if (r.ok) {
      items = r.value ?? [];
      failure = "";
    } else failure = r.error;
  }

  onMount(load);

  function startNew(name = "") {
    form = { name, value: flagProps.includes(name) ? "*" : name === "svn:eol-style" ? "native" : "", recursive: false, message: "", editing: false, error: "", busy: false };
  }

  function startEdit(p: Property) {
    form = { name: p.name, value: p.value, recursive: false, message: "", editing: true, error: "", busy: false };
  }

  function toggleKeyword(k: string) {
    if (!form) return;
    const set = new Set(keywords);
    if (set.has(k)) set.delete(k);
    else set.add(k);
    form.value = [...set].join(" ");
  }

  async function save() {
    if (!form || form.busy) return;
    const name = form.name.trim();
    const value = flagProps.includes(name) ? "*" : form.value;
    form.busy = true;
    form.error = "";
    const message = form.message.trim() || `Set ${name} on ${label}`;
    const r = await exec(`Setting ${name}`, () => svn.SetProperty({ targets: [target], name, value, recursive: form!.recursive, message }), target);
    if (!r.ok) {
      if (form) {
        form.busy = false;
        form.error = r.error;
      }
      return;
    }
    form = null;
    selected = name;
    await load();
    onchanged?.();
  }

  async function remove(p: Property, recursive: boolean) {
    const message = `Delete ${p.name} from ${label}`;
    const r = await exec(`Deleting ${p.name}`, () => svn.DeleteProperty({ targets: [target], name: p.name, value: "", recursive, message }), target);
    if (!r.ok) {
      failure = r.error;
      return;
    }
    selected = "";
    await load();
    onchanged?.();
  }
</script>

<div class="props">
  <p class="muted where" title={target}>
    {remote ? "On the server; each change is a commit of its own." : "In your working copy; changes go to the server with your next commit."}
  </p>

  {#if form}
    <form class="form" onsubmit={(e) => { e.preventDefault(); save(); }}>
      <b>{form.editing ? `Edit ${form.name}` : "New property"}</b>
      {#if !form.editing}
        <label>
          <span>Name</span>
          <!-- svelte-ignore a11y_autofocus -->
          <input bind:value={form.name} list="known-props" placeholder="svn:ignore" autofocus />
          <datalist id="known-props">{#each suggestions as k}<option value={k.name}></option>{/each}</datalist>
        </label>
      {/if}
      {#if help}<p class="muted fine">{help}</p>{/if}

      {#if flagProps.includes(form.name.trim())}
        <p class="fine">This property only has to be there; its value is always <code>*</code>.</p>
      {:else if form.name.trim() === "svn:eol-style"}
        <label><span>Line endings</span>
          <select bind:value={form.value}>{#each eolStyles as s}<option value={s}>{s}{s === "native" ? " (the computer's own)" : ""}</option>{/each}</select>
        </label>
      {:else if form.name.trim() === "svn:keywords"}
        <div class="checks">
          {#each keywordNames as k}
            <label class="check"><input type="checkbox" checked={keywords.includes(k)} onchange={() => toggleKeyword(k)} /> {k}</label>
          {/each}
        </div>
      {:else if form.name.trim() === "svn:mime-type"}
        <label><span>Type</span>
          <input bind:value={form.value} list="mime-types" placeholder="application/octet-stream" />
          <datalist id="mime-types">
            {#each ["text/plain", "application/octet-stream", "image/png", "image/jpeg", "application/pdf", "application/json", "text/xml"] as m}<option value={m}></option>{/each}
          </datalist>
        </label>
      {:else}
        <label><span>Value</span><textarea bind:value={form.value} rows="7" class="mono"></textarea></label>
      {/if}

      {#if isDir && !remote}
        <label class="check"><input type="checkbox" bind:checked={form.recursive} /> Also on everything inside this folder</label>
      {/if}
      {#if remote}
        <label><span>Commit message</span><input bind:value={form.message} placeholder={`Set ${form.name.trim() || "..."} on ${label}`} /></label>
      {/if}
      {#if form.error}<p class="bad">{form.error}</p>{/if}
      <div class="tools">
        <span class="grow"></span>
        <button type="button" onclick={() => (form = null)}>Cancel</button>
        <button class="primary" type="submit" disabled={form.busy || !form.name.trim() || form.name.trim().startsWith("-")}>{form.busy ? "Saving..." : "Save"}</button>
      </div>
    </form>
  {:else}
    <div class="table">
      <div class="row hdr"><span>Property</span><span>Value</span></div>
      <div class="rows">
        {#each items as p (p.name)}
          <button class="row" class:sel={selected === p.name} onclick={() => (selected = p.name)} ondblclick={() => startEdit(p)}>
            <span class="name">{p.name}</span>
            <span class="value">{p.value.replace(/\s+/g, " ").trim()}</span>
          </button>
        {:else}
          <p class="muted empty">{loading ? "Reading..." : failure ? "" : "No properties."}</p>
        {/each}
      </div>
    </div>
    {#if failure}<p class="bad">{failure}</p>{/if}
    <div class="tools">
      <button onclick={() => startNew()}>New...</button>
      {#each suggestions.slice(0, isDir ? 3 : 4) as k}
        <button class="quick" onclick={() => startNew(k.name)} title={k.help}>+ {k.name}</button>
      {/each}
      <span class="grow"></span>
      <button disabled={!selected} onclick={() => { const p = items.find((x) => x.name === selected); if (p) startEdit(p); }}>Edit...</button>
      <button disabled={!selected} onclick={() => { const p = items.find((x) => x.name === selected); if (p) remove(p, false); }}>Delete</button>
      {#if isDir && !remote}
        <button disabled={!selected} title="Delete it here and from everything inside" onclick={() => { const p = items.find((x) => x.name === selected); if (p) remove(p, true); }}>Delete everywhere inside</button>
      {/if}
    </div>
  {/if}
</div>

<style>
  .props { display: grid; gap: 10px; }
  .where { margin: 0; font-size: 12px; }
  .table { display: flex; flex-direction: column; height: 44vh; border: 1px solid var(--line); border-radius: var(--radius); background: var(--field); overflow: hidden; }
  .rows { overflow: auto; flex: 1; padding: 4px; }
  .row { display: grid; grid-template-columns: minmax(10em, 1fr) 2fr; gap: 12px; width: 100%; border: 0; border-radius: 4px; background: transparent; text-align: left; padding: 5px 8px; }
  .row:hover:not(:disabled) { background: var(--hover); }
  .row.sel { background: var(--accent-soft); box-shadow: inset 2px 0 0 var(--accent); }
  .hdr { font-size: 11px; font-weight: 700; letter-spacing: 0.05em; text-transform: uppercase; color: var(--muted); background: var(--surface); border-bottom: 1px solid var(--line); border-radius: 0; padding: 6px 12px; }
  .name { font-family: var(--mono); font-size: 12px; }
  .value { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--muted); font-family: var(--mono); font-size: 12px; }
  .empty { padding: 10px; margin: 0; }
  .tools { display: flex; gap: 6px; flex-wrap: wrap; align-items: center; }
  .grow { flex: 1; }
  .quick { font-family: var(--mono); font-size: 12px; }
  .form { display: grid; gap: 10px; }
  .fine { margin: 0; font-size: 12px; }
  .mono { font-family: var(--mono); font-size: 12px; }
  .checks { display: flex; flex-wrap: wrap; gap: 12px; }
  .check { display: flex; align-items: center; gap: 6px; }
  .bad { color: var(--danger); margin: 0; }
</style>
