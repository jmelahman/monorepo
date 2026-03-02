FROM archlinux:latest

RUN pacman -Syyu --noconfirm \
    && pacman -S --noconfirm base-devel binutils debugedit fakeroot sudo \
    && pacman -Scc --noconfirm

RUN useradd -m -s /bin/bash builder \
    && echo "builder ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers
