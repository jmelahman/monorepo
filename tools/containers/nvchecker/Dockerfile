### Main Stage ###
FROM archlinux:latest

RUN useradd packager -m

RUN pacman -Syyu --noconfirm \
      && \
    pacman -S --noconfirm \
      git \
      pacman-contrib \
      python-uv \
      && \
    pacman -Scc --noconfirm

RUN uv pip install --system --break-system-packages \
      git+https://github.com/lilydjwg/nvchecker@2722ccc7fef71fccf9f031d8299bc3c36736fdda \
      && \
    uv cache clean
