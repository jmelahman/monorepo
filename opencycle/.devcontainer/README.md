# Unreal Engine spike container

This repository's devcontainer builds Epic's Linux Unreal image, adds the Mesa
Vulkan ICD and `vulkan-tools`, and runs as `ue4` with the host GPU exposed.

The Dockerfile uses `USER root` only while building the image, because installing
system packages requires root. It switches back to `ue4` for normal container
and editor sessions. `ue4` is Epic's legacy account name; it is also used by
the Unreal Engine 5 image and does not mean that the image contains Unreal 4.

The Unreal spike checkout remains outside git at:

```text
~/.cache/opencycle-neural/ue
```

The profile mounts it at `/workspace/ue`, mounts the repository read-only at
`/workspace/opencycle`, and persists shader data in the named
`opencycle-ue-ddc` volume.

## Start it

The Epic image requires GHCR authentication before the first build:

```bash
docker login ghcr.io
```

Then set the host DRM group IDs and start the container from the repository
root. Because this is the repository's root devcontainer configuration, no
extra `--config` argument is needed:

```bash
export OPENCYCLE_UE_RENDER_GID="$(getent group render | cut -d: -f3)"
export OPENCYCLE_UE_VIDEO_GID="$(getent group video | cut -d: -f3)"
devcontainer up \
  --workspace-folder .
```

Verify the GPU from inside the container:

```bash
devcontainer exec \
  --workspace-folder . \
  vulkaninfo --summary
```

The existing `scripts/job.sh` and `scripts/ride.sh` can then be run from
`/workspace/ue` as described in `plans/ue-devcontainer.md`.
