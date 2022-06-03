FROM manjarolinux/base:latest

ENV SNAPCRAFT_SETUP_CORE=1

RUN pacman -Syyu --noconfirm \
      && \
    pacman -S --needed --noconfirm \
      python-pip \
      snapd \
      && \
    pacman -Scc --noconfirm

# Pre-install packages to snapify
RUN pacman -Syyu --noconfirm \
      && \
    pacman -S --needed --noconfirm \
      docker \
      firefox \
      htop \
      jq \
      nano \
      nmap \
      ripgrep \
      tree \
      vlc \
      && \
    pacman -Scc --noconfirm

RUN pip install python-snapify==0.3.0
