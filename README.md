<p align="center"><img src="build/appicon.png" width="112" alt="svnch"></p>

# svnch

A small desktop client for Subversion.

svnch requires `svn` cli commands.


## Status

Early. Only `info` works for now; checkout, update, status, commit, log
and diff come next.


## Building

You need Go, Node.js and the Wails v3 CLI.

    wails3 dev       run while developing
    wails3 build     build bin/svnch


## Subversion

If `svn` is not installed, svnch installs it on first start (apt, dnf,
pacman or zypper on Linux, Homebrew on macOS, winget on Windows).

Passwords are read from svn's own cache. For a new server, run
`svn info <url>` once in a terminal.


## License

MIT, see [LICENSE](LICENSE).
