<p align="center"><img src="build/appicon.png" width="112" alt="svnch"></p>

# svnch

A small desktop client for Subversion.

svnch is a thin window over the `svn` command line tool. It does not link
against libsvn, keep its own idea of your working copy, or rename things
that already have perfectly good names. When you click something, it runs
the same `svn` command you would have typed and reads back the `--xml`
output.

It is for people who still keep their code in a repository with a trunk,
a few branches and revision numbers that only go up.


## Status

Early. Commands are added one at a time, each with its own model and
parser.

    info        works
    checkout    next
    update
    status
    commit
    log
    diff
    add, delete, revert


## Requirements

- Go 1.26 or newer
- Node.js with npm
- Wails v3: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`
- Subversion 1.8 or newer (svnch can install it, see below)


## Building

    wails3 dev       run with live reload
    wails3 build     binary ends up in bin/svnch


## Subversion

On startup svnch looks for `svn` on your PATH. If it is missing, it asks
the system package manager to install it:

    Linux      pkexec with apt-get, dnf, pacman or zypper
    macOS      brew install subversion
    Windows    winget install Slik.Subversion

If none of these are available, svnch exits and says so. Install
Subversion by hand and start it again.

Every command runs with `--non-interactive` and `LC_ALL=C`. svn never
waits on a password prompt the window cannot show, and its errors read
the same on every machine. Credentials come from svn's own cache in
`~/.subversion/auth`, so for a new server run `svn info <url>` once in a
terminal and save the password there.


## Layout

    main.go              wires everything together
    internal/svn/        runner, installer, one file per command
    internal/service/    what the frontend can call
    frontend/            Svelte UI
    build/               packaging; appicon.svg is the logo source
