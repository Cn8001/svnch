# Builds svnch for each OS. Plain make checks this machine, then builds for it.
# Every build target runs the same check first and hands over to the Wails
# Taskfile that does the actual work. On Windows, run make from Git Bash.

ifeq ($(OS),Windows_NT)
  HOST := windows
else
  UNAME := $(shell uname -s)
  ifeq ($(UNAME),Darwin)
    HOST := darwin
  else
    HOST := linux
  endif
endif

# GTK=3 builds against GTK 3 and WebKitGTK 4.1, for distributions that have no
# WebKitGTK 6.0 yet (Ubuntu 22.04, Debian 12).
ifeq ($(GTK),3)
  LINUX_TAGS := EXTRA_TAGS=gtk3
  WEBKIT := webkit2gtk-4.1
else
  WEBKIT := webkitgtk-6.0
endif

.DEFAULT_GOAL := start
.PHONY: start help doctor dev build build-linux build-windows build-darwin build-all \
        test bindings icons version clean

## start: (plain make) check this machine and build for it
start: build
	@echo
	@echo "Built into bin/. On Windows: bin/svnch.exe, elsewhere: bin/svnch"

## help: list the targets
help:
	@echo "svnch (host: $(HOST))"
	@echo
	@sed -n 's/^## \([^:]*\): /  make \1\t/p' $(MAKEFILE_LIST) | expand -t 24

# svn itself is not listed: the app installs it on first start.
## doctor: check the tools a build needs
doctor:
	@missing=""; \
	for t in go wails3 npm; do command -v $$t >/dev/null 2>&1 || missing="$$missing $$t"; done; \
	if [ "$(HOST)" = linux ] && ! pkg-config --exists $(WEBKIT) 2>/dev/null; then missing="$$missing $(WEBKIT)"; fi; \
	if [ -n "$$missing" ]; then \
		echo "Missing:$$missing"; \
		echo "  wails3: go install github.com/wailsapp/wails/v3/cmd/wails3@latest"; \
		echo "  $(WEBKIT): install its -dev package, or try GTK=3"; \
		exit 1; \
	fi

## dev: run with live reload
dev: doctor
	wails3 task dev

## build: build for this machine's OS
build: build-$(HOST)

## build-linux: bin/svnch (GTK=3 for older distributions)
build-linux: doctor
	GOOS=linux wails3 task build $(LINUX_TAGS)

## build-windows: bin/svnch.exe, from any OS
build-windows: doctor
	GOOS=windows wails3 task build

## build-darwin: bin/svnch.app, on a Mac or through Docker
build-darwin: doctor
	GOOS=darwin wails3 task build

## build-all: Windows plus this machine's OS (the others need Docker)
ifeq ($(HOST),windows)
build-all: build-windows
else
build-all: build-$(HOST) build-windows
endif

# build/ios is a template package that does not build on its own, so vet and
# test stay on the app's packages.
## test: gofmt, vet, Go tests under -race, frontend type check
test:
	@out=$$(gofmt -l main.go internal); [ -z "$$out" ] || { echo "Not gofmt'd:"; echo "$$out"; exit 1; }
	go vet . ./internal/...
	go test -race ./internal/...
	cd frontend && npm run check

## bindings: regenerate the TypeScript bindings in frontend/bindings
bindings:
	wails3 generate bindings -clean=true -ts

## icons: regenerate the platform icons from build/appicon.png
icons:
	wails3 task common:generate:icons

## version: V=1.2.3 sets the version (build/config.yml, the one place it lives)
version:
ifneq ($(V),)
	@sed -i.bak 's/^\(  version: \)"[^"]*"/\1"$(V)"/' build/config.yml && rm build/config.yml.bak
	@wails3 task common:update:build-assets >/dev/null
endif
	@sed -n 's/^  version: "\([^"]*\)".*/\1/p' build/config.yml

## clean: remove the built binaries
clean:
	rm -rf bin
