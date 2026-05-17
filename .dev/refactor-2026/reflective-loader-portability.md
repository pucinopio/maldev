---
status: open
opened: 2026-05-17
scope: pe/packer/runtime (reflective ELF loader)
references:
  - cmd/bundle-launcher/launcher_reflective_e2e_linux_test.go (skipped on CI)
  - https://github.com/actions/runner-images/blob/ubuntu24/...  (Ubuntu 24.04 runner)
---

# Reflective ELF loader — cross-distro portability

## Symptom

`TestLauncher_E2E_ReflectiveLoadsHello` segfaults on the GitHub Actions
Ubuntu 24.04 runner (glibc 2.39, kernel 6.8). Local Fedora 44
(glibc 2.41, kernel 7.0+) — 5/5 pass.

```
launcher_reflective_e2e_linux_test.go:79: reflective run: signal: segmentation fault (core dumped)
    output: ""
```

Output is empty → the segfault happens **before** the loaded payload
writes its `"hello from packer"` marker, so the failure is in the
loader (auxv patching, entry-jump stack shape, mprotect ordering),
not in the static-PIE fixture.

## Why it's distro-sensitive

`pe/packer/runtime.Run` performs a *userspace* ELF load — mmap'ing
PT_LOADs, applying `R_X86_64_RELATIVE` relocations, patching auxv to
point at the loaded image, then jumping to entry on a fake stack. Each
step has a kernel + libc dependency:

- **auxv layout** — kernel 6.x emits the same `AT_*` enum values as
  7.x, but the *ordering* and presence of optional entries
  (`AT_BASE_PLATFORM`, `AT_EXECFN`) varies. Our patching code rewrites
  `AT_PHDR / AT_ENTRY / AT_PHNUM / AT_BASE` in place and relies on the
  ones we don't touch staying valid.
- **glibc startup** — glibc 2.39 reads `AT_RANDOM` early; if the
  patched auxv leaves the original pointer pointing into the
  *launcher's* image region we just munmap'd, the canary read
  segfaults.
- **vDSO mapping** — Ubuntu 24.04 ships a different vDSO layout; if
  the loaded payload's libc tries to use the vDSO before the patched
  auxv `AT_SYSINFO_EHDR` is consistent, a syscall through vDSO
  segfaults.

## Repro

```bash
# On a Fedora dev box (passes):
go test -run TestLauncher_E2E_ReflectiveLoadsHello -count=5 ./cmd/bundle-launcher/

# Repro the CI failure locally — spin up the ubuntu20.04- libvirt VM
# (rough analog), or pull ubuntu:24.04 in podman and rebuild + test.
# (Confirmed segfaulting on the GH Actions runner per build run
# 25999420718 against commit ed076144.)
```

## Investigation queue

1. Capture a core dump on Ubuntu 24.04 and identify which step in
   `Run` faults (`mprotect` boundary vs entry jump vs first libc
   instruction).
2. Compare auxv contents between Fedora and Ubuntu kernels: dump via
   `auxv_print` after `Prepare` and diff.
3. If auxv ordering is the cause, switch from in-place patch to
   "rebuild the auxv array from a known-good template".

## Current mitigation

`launcher_reflective_e2e_linux_test.go` skips when
`GITHUB_ACTIONS=true` so the build CI stops flapping. Local runs are
unaffected. **The fix is real work** — slot in alongside slice 2 of
the BOF revamp or earlier if anyone trips on this again.
