# maldev build pipeline — cross-platform (Linux / macOS / Windows w/ Git-Bash, WSL, or MSYS2)
#
# Quick reference:
#   make tools           build every cmd/* into ./bin/
#   make <tool>          build one utility (e.g. make license-manager, make packer)
#   make build           rshell -> $(BINARY) at repo root (legacy default)
#   make release         OPSEC rshell build (garble + strip + trimpath)
#   make debug           debug-tagged rshell build with logging enabled
#   make test            non-intrusive test suite
#   make test-intrusive  shellcode-execution tests (MALDEV_INTRUSIVE=1)
#   make verify          build + test + Linux cross-build smoke
#   make cross-linux     cross-compile rshell for Linux amd64
#   make cross-tools     build every tool for Linux amd64 into ./bin/linux/
#   make packer-demo     run the packer elevation tour (Linux-only)
#   make clean           rm bin/, implant_linux, stray *.exe
#   make help            print this help
#
# Windows note: needs GNU make + a POSIX-ish shell (Git-Bash, WSL, MSYS2).
# Native cmd.exe is not supported.

# ── Host OS detection ────────────────────────────────────────────────────────
# MSYS/Git-Bash make strips $(OS) from its environment, so we detect Windows
# via either $(OS) (Windows-native make) or $(MSYSTEM) (MSYS/MinGW).
ifeq ($(OS),Windows_NT)
    HOST_OS  := windows
else ifneq ($(MSYSTEM),)
    HOST_OS  := windows
else
    HOST_OS  := $(shell uname -s | tr '[:upper:]' '[:lower:]')
endif

ifeq ($(HOST_OS),windows)
    EXE := .exe
    # MSYS/Git-Bash strips USERPROFILE/TMP/GOPATH from make's environment, so
    # the Go toolchain falls back to system defaults that aren't writable.
    # Pin all Go state to repo-local .gocache/ (writable, gitignored). This
    # works identically from bash, cmd, and PowerShell.
    export GOTMPDIR   := $(CURDIR)/.gocache/tmp
    export GOCACHE    := $(CURDIR)/.gocache/cache
    export GOPATH     := $(CURDIR)/.gocache/gopath
    export GOMODCACHE := $(CURDIR)/.gocache/gopath/pkg/mod
else
    EXE :=
endif

# ── Flags ────────────────────────────────────────────────────────────────────
BIN      ?= bin
GOFLAGS  := -trimpath
LDFLAGS  := -s -w -buildid=
TAGS     ?=

# rshell is a GUI implant — hide the console window on Windows builds.
RSHELL_LDFLAGS := $(LDFLAGS) -H windowsgui

# Legacy rshell-only default (preserved for backwards compatibility).
BINARY   ?= implant$(EXE)
CMD      ?= ./cmd/rshell

# ── Tools matrix ─────────────────────────────────────────────────────────────
# All cmd/ subdirectories worth shipping. Add new ones here.
TOOLS := \
    bof-runner \
    bundle-launcher \
    cert-snapshot \
    hashgen \
    license-manager \
    license-test \
    memscan-harness \
    memscan-mcp \
    memscan-server \
    packer \
    packer-vis \
    packerscope \
    privesc-e2e \
    rshell \
    sleepmask-demo \
    test-report \
    tui-orphan-scan \
    tui-snap \
    vmtest

.PHONY: all build tools release debug test test-intrusive verify cross-linux \
        cross-tools install-garble packer-demo clean help $(TOOLS) \
        $(addprefix cross-,$(TOOLS))

# ── Default target ───────────────────────────────────────────────────────────
all: tools

help:
	@echo "Targets:"
	@echo "  make tools           build every cmd/* into ./$(BIN)/"
	@echo "  make <tool>          build one utility into ./$(BIN)/  (see list below)"
	@echo "  make build           rshell -> $(BINARY) (legacy)"
	@echo "  make release         OPSEC rshell build (garble)"
	@echo "  make test            run all tests"
	@echo "  make test-intrusive  run intrusive tests"
	@echo "  make verify          build + test + Linux cross-build"
	@echo "  make cross-linux     rshell for Linux amd64"
	@echo "  make cross-tools     all tools for Linux amd64 (./$(BIN)/linux/)"
	@echo "  make packer-demo     packer elevation tour (Linux-only)"
	@echo "  make clean           remove $(BIN)/ and stray binaries"
	@echo ""
	@echo "Tools: $(TOOLS)"

# ── Output directories ────────────────────────────────────────────────────────
$(BIN):
	@mkdir -p $(BIN)

$(BIN)/linux: | $(BIN)
	@mkdir -p $(BIN)/linux

# Go state sink — ensures GOTMPDIR/GOCACHE/GOPATH/GOMODCACHE directories exist
# on Windows where the bash-inherited paths don't resolve.
.gocache:
	@mkdir -p .gocache/tmp .gocache/cache .gocache/gopath/pkg/mod

# ── Per-tool build targets ───────────────────────────────────────────────────
# Each $(t) target builds ./cmd/$(t) → ./$(BIN)/$(t)[.exe]
# rshell gets the Windows GUI ldflag; everything else takes plain LDFLAGS.
define TOOL_template
$(1): | $$(BIN) .gocache
	@echo "build $(1) -> $$(BIN)/$(1)$$(EXE)"
	@go build $$(GOFLAGS) \
		-ldflags="$$(if $$(filter rshell,$(1)),$$(RSHELL_LDFLAGS),$$(LDFLAGS))" \
		-tags="$$(TAGS)" \
		-o $$(BIN)/$(1)$$(EXE) ./cmd/$(1)
endef
$(foreach t,$(TOOLS),$(eval $(call TOOL_template,$(t))))

# Build every tool.
tools: $(TOOLS)
	@echo "$(words $(TOOLS)) tools built into $(BIN)/"

# ── Linux cross-build matrix ─────────────────────────────────────────────────
# Each cross-$(t) target builds ./cmd/$(t) for linux/amd64 → ./$(BIN)/linux/$(t)
# rshell drops the Windows-only -H windowsgui flag on Linux.
define CROSS_template
cross-$(1): | $$(BIN)/linux
	@echo "cross-build $(1) -> $$(BIN)/linux/$(1)"
	@GOOS=linux GOARCH=amd64 go build $$(GOFLAGS) \
		-ldflags="$$(LDFLAGS)" \
		-tags="$$(TAGS)" \
		-o $$(BIN)/linux/$(1) ./cmd/$(1)
endef
$(foreach t,$(TOOLS),$(eval $(call CROSS_template,$(t))))

cross-tools: $(addprefix cross-,$(TOOLS))
	@echo "$(words $(TOOLS)) tools cross-built for linux/amd64 into $(BIN)/linux/"

# ── Legacy rshell-at-root targets ────────────────────────────────────────────
build:
	go build $(GOFLAGS) -ldflags="$(RSHELL_LDFLAGS)" -o $(BINARY) $(CMD)

# OPSEC release build (requires: go install mvdan.cc/garble@latest)
# - garble: randomizes symbols, encrypts strings, strips pclntab info
# - -literals: encrypts all string literals
# - -tiny: removes panic messages + print runtime
# - -seed=random: different obfuscation per build
release:
	CGO_ENABLED=0 garble -literals -tiny -seed=random \
		build $(GOFLAGS) -ldflags="$(RSHELL_LDFLAGS)" -tags="$(TAGS)" \
		-o $(BINARY) $(CMD)

debug:
	go build $(GOFLAGS) -tags=debug -ldflags="-s -w" -o $(BINARY) $(CMD)

cross-linux:
	GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o implant_linux $(CMD)

# ── Tests ────────────────────────────────────────────────────────────────────
# `go list ./...` already skips ignore/ because it has no Go files,
# but we keep the explicit guard for safety.
test:
	go test ./... -count=1

test-intrusive:
	MALDEV_INTRUSIVE=1 go test ./... -count=1

# Full verification: host build + tests + Linux cross-build.
verify:
	go build ./...
	go test ./... -count=1
	GOOS=linux GOARCH=amd64 go build ./...
	@echo "All checks passed."

# ── TUI snapshot targets ─────────────────────────────────────────────────────
# Requires: go install github.com/charmbracelet/freeze@latest
# Output:   ignore/snapshots/<view>.png

TUI_VIEWS := dashboard licenses issuers recipients identities revocation servers totp audit settings

.PHONY: snap snap-all

# Single-view snapshot: make snap VIEW=dashboard
# Produces ignore/snapshots/<view>.svg (SVG — PNG path crashes on Windows w/ freeze v0.2.2).
VIEW ?= dashboard
snap: tui-snap
	@bash scripts/tui-snap.sh $(VIEW)

# All views in sequence.
snap-all: tui-snap
	@for v in $(TUI_VIEWS); do bash scripts/tui-snap.sh $$v; done

# Orphan-hint scan: report '[X]' hints visible in the rendered TUI that have
# no matching key handler in their screen source. Reveals visual promises
# the code never delivers (typical: dashboard Raccourcis cards offering
# shortcuts that aren't wired).
.PHONY: orphans
orphans: tui-snap tui-orphan-scan
	@./$(BIN)/tui-orphan-scan$(EXE)

# ── VHS regression tapes ─────────────────────────────────────────────────────
# Renders a recorded GIF of the TUI under deterministic input. Requires
# vhs (~/go/bin/vhs), ttyd, ffmpeg. Outputs land under tapes/out/.
.PHONY: tapes tape-themes tape-wizard-step3 tape-smoke tape-screens
tapes: tape-themes tape-wizard-step3 tape-smoke tape-screens

# Per-screen tapes — one .gif per top-level view under tapes/out/screens/.
# Script regenerates the .tape files so adding a screen to the list
# is one-line.
tape-screens: tui-snap
	@bash scripts/gen-screen-tapes.sh

tape-themes: tui-snap license-manager
	@mkdir -p tapes/out
	@vhs tapes/themes.tape

tape-wizard-step3: tui-snap
	@mkdir -p tapes/out
	@vhs tapes/wizard-step3.tape

tape-smoke: tui-snap
	@mkdir -p tapes/out
	@vhs tapes/dashboard-smoke.tape

# ── Tooling helpers ──────────────────────────────────────────────────────────
install-garble:
	go install mvdan.cc/garble@latest

# Packer elevation tour — runs every shipped pack variant in one session
# (raw min-ELF, all-asm bundle, Go launcher default, Go launcher reflective).
# Linux-only because it uses memfd / static-PIE.
packer-demo:
	@if [ "$$(uname)" != "Linux" ]; then \
	  echo "packer-demo: Linux-only (uses memfd / static-PIE)."; exit 1; \
	fi
	@bash scripts/packer-demo.sh

# ── Clean ────────────────────────────────────────────────────────────────────
clean:
	@rm -rf $(BIN) implant_linux 2>/dev/null || true
	@rm -f *.exe 2>/dev/null || true
	@echo "cleaned $(BIN)/ + stray binaries"
