### Main Stage ###
FROM archlinux:latest

RUN useradd packager -m

RUN pacman -Syyu --noconfirm \
      && \
    pacman -S --noconfirm \
      git \
      github-cli \
      pacman-contrib \
      python-uv \
      && \
    pacman -Scc --noconfirm

RUN uv pip install --system --break-system-packages \
      git+https://github.com/lilydjwg/nvchecker@372fce4445159ebd2cab8dab4f3e40e20a54ee9a \
      && \
    uv cache clean

