import type { StatusEntry } from "../../bindings/github.com/Cn8001/svnch/internal/svn";

export {
  SVNService as svn,
  AppService,
  UpdateService,
  SetupService,
  ServerService,
} from "../../bindings/github.com/Cn8001/svnch/internal/service";
export type { Server } from "../../bindings/github.com/Cn8001/svnch/internal/servers";
export * from "../../bindings/github.com/Cn8001/svnch/internal/svn";
export type { Info as AppInfo } from "../../bindings/github.com/Cn8001/svnch/internal/appinfo";

export type MenuItem =
  | { separator: true }
  | { label: string; action: () => void; disabled?: boolean; hint?: string; separator?: false };

// The letters svn status prints in a terminal, so the list reads the same as the command line.
const itemCodes: Record<string, string> = {
  added: "A",
  conflicted: "C",
  deleted: "D",
  external: "X",
  ignored: "I",
  incomplete: "!",
  merged: "G",
  missing: "!",
  modified: "M",
  normal: " ",
  obstructed: "~",
  replaced: "R",
  unversioned: "?",
};

export function statusCode(e: StatusEntry): string {
  const item = itemCodes[e.item] ?? "?";
  const props = e.props === "modified" ? "M" : e.props === "conflicted" ? "C" : " ";
  return item + props + (e.treeConflicted ? "C" : " ");
}

export function isVersioned(e: StatusEntry): boolean {
  return e.item !== "unversioned" && e.item !== "ignored";
}

export function hasChanges(e: StatusEntry): boolean {
  return isVersioned(e) && (e.item !== "normal" || e.props === "modified");
}

export function relative(wc: string, path: string): string {
  if (path === wc) return ".";
  if (path.startsWith(wc) && /[\\/]/.test(path[wc.length])) return path.slice(wc.length + 1);
  return path;
}

export function join(dir: string, name: string): string {
  const sep = dir.includes("\\") && !dir.includes("/") ? "\\" : "/";
  return dir.replace(/[\\/]+$/, "") + sep + name;
}

// The Go side maps its errors to a stable code (service.MarshalError), which Wails puts on the error's cause.
export function failureCode(e: unknown): string {
  const cause = (e as { cause?: { code?: unknown } } | null)?.cause;
  return typeof cause?.code === "string" ? cause.code : "";
}

// Wails hands Go errors over as an Error whose message is the full wrapped chain; the last svn line is the useful part.
export function errorText(e: unknown): string {
  const text = e instanceof Error ? e.message : String(e);
  const svnLines = text.split("\n").filter((l) => l.includes("svn: E"));
  return (svnLines.at(-1) ?? text).trim();
}
