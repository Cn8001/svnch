# Third-party notices

svnch bundles the software below. Every package is under a permissive licence;
none is copyleft. The full licence text and copyright of every package svnch
ships, including those the packages below pull in, are under About >
Third-party licences, and in `THIRD_PARTY_LICENSES.json` (written by
`make licenses`).

## Compiled into the binary (Go)

The modules svnch requires itself (`go.mod`).

| Module | Licence |
|---|---|
| Go standard library | BSD-3-Clause |
| github.com/wailsapp/wails/v3 | MIT |

## Bundled in the frontend

| Package | Licence |
|---|---|
| svelte | MIT |
| @wailsio/runtime | MIT |

## Pulled in by the above

Not chosen by svnch but compiled in with it, through Wails.

| Module | Licence |
|---|---|
| github.com/adrg/xdg | MIT |
| github.com/godbus/dbus/v5 (Linux) | BSD-2-Clause |
| github.com/go-ole/go-ole (Windows) | MIT |
| github.com/mattn/go-isatty, go-colorable | MIT |
| golang.org/x/sys, golang.org/x/mod | BSD-3-Clause |

## Logo

The svnch logo and its pixel lettering were drawn for svnch and are under the
same MIT licence as the code.

## Run, not bundled

svnch starts these as separate programs; it does not ship or link them:
`svn`, Apache Subversion (Apache-2.0), and on first start, when svn is missing,
the system package manager that installs it: `apt-get`, `dnf`, `pacman` or
`zypper` through `pkexec` on Linux, Homebrew on macOS, and `winget` with
SlikSVN on Windows. Running a program as its own process does not place svnch
under that program's licence.
