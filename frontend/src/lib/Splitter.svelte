<script lang="ts">
  // A bar between two panes that the pointer drags to share the space differently. It reports the size of the pane
  // before it; the caller lays the panes out and remembers the size.
  let {
    size,
    min,
    max,
    vertical = false,
    after = false,
    onresize,
  }: {
    size: number; // px of the pane before the bar
    min: number;
    max: number;
    vertical?: boolean; // a bar between panes stacked top and bottom
    after?: boolean; // size is the pane after the bar, which grows as the bar moves back
    onresize: (size: number) => void;
  } = $props();

  let dragging = $state(false);

  function down(e: PointerEvent) {
    e.preventDefault();
    const bar = e.currentTarget as HTMLElement;
    bar.setPointerCapture(e.pointerId);
    const start = vertical ? e.clientY : e.clientX;
    const from = size;
    dragging = true;
    const move = (ev: PointerEvent) => {
      const delta = ((vertical ? ev.clientY : ev.clientX) - start) * (after ? -1 : 1);
      onresize(Math.max(min, Math.min(max, from + delta)));
    };
    const up = () => {
      dragging = false;
      bar.removeEventListener("pointermove", move);
      bar.removeEventListener("pointerup", up);
      bar.removeEventListener("pointercancel", up);
    };
    bar.addEventListener("pointermove", move);
    bar.addEventListener("pointerup", up);
    bar.addEventListener("pointercancel", up);
  }

  function key(e: KeyboardEvent) {
    const step = e.shiftKey ? 40 : 10;
    const back = vertical ? "ArrowUp" : "ArrowLeft";
    const on = vertical ? "ArrowDown" : "ArrowRight";
    const dir = after ? -1 : 1;
    if (e.key === back) onresize(Math.max(min, Math.min(max, size - step * dir)));
    else if (e.key === on) onresize(Math.max(min, Math.min(max, size + step * dir)));
    else return;
    e.preventDefault();
  }
</script>

<!-- A focusable separator is a widget in ARIA, moved with the arrow keys, which the checker does not know. -->
<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<div
  class="splitter"
  class:vertical
  class:dragging
  role="separator"
  aria-orientation={vertical ? "horizontal" : "vertical"}
  aria-valuenow={Math.round(size)}
  aria-valuemin={min}
  aria-valuemax={max}
  tabindex="0"
  title="Drag to resize"
  onpointerdown={down}
  onkeydown={key}
></div>

<style>
  .splitter { position: relative; z-index: 2; flex: none; width: 5px; margin: 0 -2px; cursor: col-resize; background: transparent; touch-action: none; }
  .splitter::after { content: ""; position: absolute; inset: 0 2px; background: var(--line); transition: background 0.12s; }
  .splitter.vertical { width: auto; height: 5px; margin: -2px 0; cursor: row-resize; }
  .splitter.vertical::after { inset: 2px 0; }
  .splitter:hover::after, .splitter.dragging::after, .splitter:focus-visible::after { background: var(--accent); }
  .splitter:focus-visible { outline: none; }
</style>
