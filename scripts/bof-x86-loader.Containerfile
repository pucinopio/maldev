# Container image for reproducible builds of
# runtime/bof/internal/x86loader/bof_x86_loader.x86.dll.
#
# Used by scripts/build-bof-x86-loader.sh as a fallback when the
# host doesn't have i686-w64-mingw32-gcc on PATH. Pinned to
# fedora:42 so the toolchain version stays predictable across
# rebuild sessions.

FROM fedora:42

RUN dnf install -y mingw32-gcc \
    && dnf clean all

WORKDIR /src
