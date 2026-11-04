import { getContext, setContext } from "svelte";

export type ThemeName = "dark" | "light" | "system";

const KEY = "svnch.theme";

// Theme follows the choice in Settings, and the desktop's own setting when that choice is System.
export class Theme {
  name = $state<ThemeName>(Theme.read());
  private systemLight = $state(matchMedia("(prefers-color-scheme: light)").matches);
  light = $derived(this.name === "light" || (this.name === "system" && this.systemLight));

  constructor() {
    matchMedia("(prefers-color-scheme: light)").addEventListener("change", (e) => (this.systemLight = e.matches));
  }

  private static read(): ThemeName {
    try {
      const t = localStorage.getItem(KEY);
      return t === "light" || t === "system" ? t : "dark";
    } catch {
      return "dark";
    }
  }

  set = (t: ThemeName) => {
    this.name = t;
    try {
      localStorage.setItem(KEY, t);
    } catch {}
  };
}

const key = Symbol("theme");

export function provideTheme(t: Theme) {
  setContext(key, t);
}

export function useTheme(): Theme {
  return getContext<Theme>(key);
}
