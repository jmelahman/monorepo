FROM manjarolinux/base:latest

ENV SNAPCRAFT_SETUP_CORE=1

RUN pacman -Syyu --noconfirm
RUN pacman -S --needed --noconfirm \
      python-pip \
      snapd

# Pre-install packages to snapify
RUN pacman -S --needed --noconfirm \
      docker \
      firefox \
      htop \
      jq \
      nano \
      nmap \
      ripgrep \
      tree \
      vlc

RUN pip install python-snapify==0.3.0
