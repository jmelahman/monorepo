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
      nvchecker[pypi] \
      && \
    uv cache clean
