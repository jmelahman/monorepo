### Prebuild Stage ###
FROM archlinux:base-devel as prebuild

RUN pacman -Sy --needed --noconfirm
RUN pacman -S --needed --noconfirm git sudo

RUN useradd builduser -m
RUN passwd -d builduser
RUN echo 'builduser ALL=(ALL) ALL' >> /etc/sudoers

RUN cd /opt && \
    git clone https://aur.archlinux.org/yay-git.git && \
    chown -R builduser:builduser ./yay-git
USER builduser
RUN cd /opt/yay-git && \
    makepkg -s --noconfirm

### Main Stage ###
FROM archlinux:latest

RUN pacman -Syyu --noconfirm
RUN pacman -S --noconfirm \
      git \
      python \
      wget

RUN wget https://raw.githubusercontent.com/gdraheim/docker-systemctl-replacement/master/files/docker/systemctl3.py \
      -O /usr/local/sbin/systemctl && \
    chmod +x /usr/local/sbin/systemctl

COPY --from=prebuild /opt/yay-git/pkg/yay-git/usr/bin/yay /usr/local/bin/yay
