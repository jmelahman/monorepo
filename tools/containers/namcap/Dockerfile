### Main Stage ###
FROM archlinux:latest

RUN pacman -Syyu --noconfirm \
      && \
    pacman -S --noconfirm \
      namcap \
      && \
    pacman -Scc --noconfirm
