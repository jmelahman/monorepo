# Porter

`runtainer` is a lightweight, userspace container runtime that runs OCI images directly atop the host filesystem.
Instead of full isolation, it transparently overlays the container image filesystem over top the host.
This enables running containerized apps with minimal overhead (fast) while preserving access to host resources where appropriate (reduce configuration complexity).

Goals

    ✅ Run processes from standard OCI images without root.

    ✅ Implement merged view: container > host.

    ✅ Avoid full sandboxing; focus on practical compatibility.

    ✅ Favor simplicity and portability (no kernel modules).

    ✅ No daemon, no cgroups, no external dependencies.

Milestones
🧪 0.1 — Proof of Concept

Pull/unpack OCI image to local directory.

Implement FUSE-based merged filesystem (container over host).

    Execute a binary inside merged view (manual chroot + unshare).

🔧 0.2 — Minimal Runtime

Implement basic overlay-runtime run <image> CLI.

Support image unpacking from OCI tarball.

Unprivileged operation via unshare and FUSE.

    Logging and error handling.

🚀 0.3 — Integration Features

Support bind-mounting volumes.

Support user remapping (UID/GID).

    Configurable fallbacks: allow per-path override (e.g. ignore host /etc).

📦 0.4 — Image Support

Direct pull from remote OCI registry (e.g. docker.io/library/alpine).

Optional layer caching.

    Clean image lifecycle management.

🧱 Future Ideas

Isolation modes (optional namespaces: net, pid).

Compatibility with containerd / CRI shims (stretch).

Interactive REPL to explore/exec in merged container.
