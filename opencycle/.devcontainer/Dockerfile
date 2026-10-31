# The monorepo's devcontainer (templates/fullstack-template/.devcontainer in
# https://github.com/jmelahman/monorepo) plus Godot and what it needs to run
# without a screen. docker-bake.hcl builds the base and this in one pass, so
# this always sits on the base from the same commit.
#
# The context is the template root, cut down by .dockerignore to the three
# provisioning scripts and two lists, so editing the game never rebuilds this.
FROM ghcr.io/jmelahman/devcontainer:latest

USER root

COPY scripts/env.sh scripts/versions.env scripts/setup.sh /opt/godot-setup/scripts/
COPY .devcontainer/packages.txt /opt/godot-setup/.devcontainer/

# Godot goes into /usr/local. The addon archives are fetched too, into a cache
# outside ~/.cache (a volume, which would hide it), so the postCreate
# `scripts/setup.sh` unpacks them without touching the network. The addons
# unpacked here are thrown away: they belong in the project, not the image.
# gdtoolkit goes in whole, for scripts/lint.sh.
ENV GODOT_DOWNLOADS=/opt/godot-downloads
RUN /opt/godot-setup/scripts/setup.sh system \
  && GODOT_PREFIX=/usr/local /opt/godot-setup/scripts/setup.sh godot addons \
  && rm -rf /opt/godot-setup/addons /opt/godot-setup/.godot-home \
    /opt/godot-downloads/Godot_*.zip /var/lib/apt/lists/* \
  && chown -R dev:dev /opt/godot-downloads \
  && . /opt/godot-setup/scripts/versions.env \
  && UV_TOOL_DIR=/opt/uv-tools UV_TOOL_BIN_DIR=/usr/local/bin \
    uv tool install "gdtoolkit==$GDTOOLKIT_VERSION"

# What a pin bump downloads at runtime, read by the base image's
# init-firewall.sh.
COPY .devcontainer/allowed-domains.txt /usr/local/share/devcontainer/allowed-domains.d/godot.txt

USER dev
