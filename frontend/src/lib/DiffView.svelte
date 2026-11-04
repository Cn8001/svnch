<script lang="ts">
  let { title, text, loading = false }: { title: string; text: string; loading?: boolean } = $props();

  type Line = { kind: "meta" | "hunk" | "add" | "del" | "ctx"; old: string; new: string; text: string };

  // Line numbers come from each hunk header, so the gutter matches the file as the reader knows it.
  let lines = $derived.by(() => {
    const out: Line[] = [];
    let o = 0;
    let n = 0;
    for (const t of text.split("\n")) {
      const h = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/.exec(t);
      if (h) {
        o = Number(h[1]);
        n = Number(h[2]);
        out.push({ kind: "hunk", old: "", new: "", text: t });
      } else if (t.startsWith("Index: ") || t.startsWith("====") || t.startsWith("--- ") || t.startsWith("+++ ") || t.startsWith("\\")) {
        out.push({ kind: "meta", old: "", new: "", text: t });
      } else if (t.startsWith("+")) {
        out.push({ kind: "add", old: "", new: String(n++), text: t });
      } else if (t.startsWith("-")) {
        out.push({ kind: "del", old: String(o++), new: "", text: t });
      } else if (t !== "" || o > 0) {
        out.push({ kind: "ctx", old: String(o++), new: String(n++), text: t });
      }
    }
    while (out.length && out.at(-1)!.kind === "ctx" && out.at(-1)!.text === "") out.pop();
    return out;
  });
</script>

<div class="diff">
  <div class="head">
    <span class="title" {title}>{title || "Diff"}</span>
    {#if loading}<span class="muted">loading...</span>{/if}
  </div>
  <div class="body">
    {#if !text}
      <p class="empty muted">{loading ? "" : "Select a file to see its changes."}</p>
    {:else}
      <pre>{#each lines as l}<div class="line {l.kind}"><span class="num">{l.old}</span><span class="num">{l.new}</span><span class="txt">{l.text || " "}</span></div>{/each}</pre>
    {/if}
  </div>
</div>

<style>
  .diff { display: flex; flex-direction: column; min-height: 0; height: 100%; }
  .head {
    display: flex;
    justify-content: space-between;
    gap: 8px;
    padding: 2px 8px;
    border-bottom: 2px solid var(--ink);
    background: var(--cream);
    font-weight: bold;
  }
  .title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .body { flex: 1; overflow: auto; background: var(--field); }
  pre { min-width: max-content; }
  .line { display: grid; grid-template-columns: 4.5ch 4.5ch 1fr; }
  .num {
    padding-right: 6px;
    text-align: right;
    color: var(--muted);
    border-right: 1px solid rgba(36, 33, 30, 0.2);
    user-select: none;
  }
  .txt { padding-left: 8px; padding-right: 12px; white-space: pre; }
  .add { background: var(--add-bg); color: var(--add-ink); }
  .del { background: var(--del-bg); color: var(--del-ink); }
  .hunk { background: var(--cream); color: var(--brick); }
  .meta { color: var(--muted); font-weight: bold; }
  .empty { padding: 12px; margin: 0; }
</style>
