<script lang="ts">
  import { useSession } from "../session.svelte";
  import CheckoutDialog from "./CheckoutDialog.svelte";
  import MoveDialog from "./MoveDialog.svelte";
  import ConfirmDialog from "./ConfirmDialog.svelte";
  import MergeDialog from "./MergeDialog.svelte";
  import ShelveDialog from "./ShelveDialog.svelte";
  import UnshelveDialog from "./UnshelveDialog.svelte";
  import LogDialog from "./LogDialog.svelte";
  import HelpDialog from "./HelpDialog.svelte";
  import TextDialog from "./TextDialog.svelte";
  import AboutDialog from "./AboutDialog.svelte";
  import SettingsDialog from "./SettingsDialog.svelte";
  import ReposDialog from "./ReposDialog.svelte";
  import BranchDialog from "./BranchDialog.svelte";
  import SwitchDialog from "./SwitchDialog.svelte";
  import ChangelistDialog from "./ChangelistDialog.svelte";
  import LockDialog from "./LockDialog.svelte";
  import RelocateDialog from "./RelocateDialog.svelte";
  import ImportDialog from "./ImportDialog.svelte";
  import PropsDialog from "./PropsDialog.svelte";
  import ConflictsDialog from "./ConflictsDialog.svelte";
  import DetailsDialog from "./DetailsDialog.svelte";
  import SvnDialog from "./SvnDialog.svelte";
  import CertPrompt from "./CertPrompt.svelte";
  import LoginPrompt from "./LoginPrompt.svelte";

  const s = useSession();
  let d = $derived(s.dialog);
</script>

<!-- A key per dialog value, so opening the same kind again starts it fresh rather than reusing the old form. -->
{#key d}
  {#if d?.kind === "checkout"}<CheckoutDialog url={d.url} path={d.path} />
  {:else if d?.kind === "move"}<MoveDialog path={d.path} />
  {:else if d?.kind === "confirm"}<ConfirmDialog title={d.title} text={d.text} ok={d.ok} run={d.run} />
  {:else if d?.kind === "merge"}<MergeDialog />
  {:else if d?.kind === "shelve"}<ShelveDialog />
  {:else if d?.kind === "unshelve"}<UnshelveDialog />
  {:else if d?.kind === "log"}<LogDialog target={d.target} purpose={d.purpose} />
  {:else if d?.kind === "help"}<HelpDialog topic={d.topic} />
  {:else if d?.kind === "text"}<TextDialog title={d.title} text={d.text} />
  {:else if d?.kind === "about"}<AboutDialog />
  {:else if d?.kind === "settings"}<SettingsDialog />
  {:else if d?.kind === "repos"}<ReposDialog at={d.at} />
  {:else if d?.kind === "svn"}<SvnDialog />
  {:else if d?.kind === "details"}<DetailsDialog />
  {:else if d?.kind === "branch" && s.info}<BranchDialog />
  {:else if d?.kind === "switch" && s.info}<SwitchDialog />
  {:else if d?.kind === "changelist"}<ChangelistDialog paths={d.paths} />
  {:else if d?.kind === "conflicts"}<ConflictsDialog first={d.first} />
  {:else if d?.kind === "props"}<PropsDialog target={d.target} label={d.label} isDir={d.isDir} />
  {:else if d?.kind === "lock"}<LockDialog paths={d.paths} />
  {:else if d?.kind === "relocate"}<RelocateDialog />
  {:else if d?.kind === "import"}<ImportDialog path={d.path} />
  {/if}
{/key}

<CertPrompt />
<LoginPrompt />
