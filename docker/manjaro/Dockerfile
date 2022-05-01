FROM manjarolinux/base:latest

ENV SNAPCRAFT_SETUP_CORE=1

RUN pacman -Syyu --noconfirm
RUN pacman -S --needed --noconfirm \
      snapd \
      wget

RUN wget https://raw.githubusercontent.com/gdraheim/docker-systemctl-replacement/master/files/docker/systemctl3.py \
      -O /usr/local/sbin/systemctl && \
    chmod +x /usr/local/sbin/systemctl

RUN ls /run/snapd.socket
RUN /usr/local/sbin/systemctl enable snapd.service
RUN /usr/local/sbin/systemctl enable snapd.socket
