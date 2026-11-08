# Unreal Engine session image

Epic's Linux Unreal image, plus a Mesa 25 Vulkan ICD, `vulkan-tools`, bun, Claude
Code and OpenCode. Kisak's stable Mesa backport is there because the Ubuntu 22.04
base image's Mesa doesn't support the local AMD Strix GPU.

`USER root` appears only while building, because installing packages needs it.
`ue4` is Epic's legacy account name and is used by the Unreal Engine 5 image too.

opencycle's kanban sessions run in it: `opencycle/.config/kanban/config.toml` names
the image and adds the GPU, display, Unreal checkout (`~/.cache/opencycle-neural/ue`
at `/workspace/ue`) and shader-cache mounts.

## Build it

Epic's image needs a GHCR login. It is amd64 only, so it isn't in the default bake group.

```bash
docker login ghcr.io
docker buildx bake unreal-devcontainer            # from the repository root
docker buildx bake unreal-devcontainer --push     # to publish it for kanban
```

`.github/workflows/devcontainer.yml` builds and pushes it on changes to this directory.
GITHUB_TOKEN can't pull Epic's image, so that job logs in with the `EPIC_GHCR_TOKEN`
repository secret: a personal access token with `read:packages` and `write:packages`
from an account in the EpicGames organization.

Kanban applies only `--cap-add`, `--cap-drop`, `--privileged`, `--network` and `--init`
from `run_args`, so `--group-add`, `--shm-size` and `--device` can't be set from the
config. The GPU comes through the `/dev/dri` bind mount.
