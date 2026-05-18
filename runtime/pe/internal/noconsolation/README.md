# No-Consolation embedded object

This directory holds the compiled No-Consolation BOF object file
(`NoConsolation.x64.o`) that `runtime/pe` embeds via `//go:embed`
when built with the `pe_noconsolation` build tag.

The `.o` is **not** vendored upstream — fortra/No-Consolation
publishes no release artefacts. Produce it locally with:

```bash
bash tools/no-consolation-build.sh
```

The script clones the upstream repo at a pinned commit, compiles
with `x86_64-w64-mingw32-gcc`, and drops `NoConsolation.x64.o`
plus `LICENSE` (MIT) into this directory. Both files are
git-ignored by default — operators choose whether to commit
them per-implant.

## Why a build tag

The default `go build` stays reproducible on machines that
don't have mingw installed and doesn't pull a third-party
binary into the implant unless the operator opted in. Mirrors
the discipline used by `kernel/driver/rtcore64` for the BYOVD
driver embed.
