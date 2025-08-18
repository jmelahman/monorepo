# Docker

## Dirty Images

To create an image from a running container, list the running containers,

```shell
$ docker ps
CONTAINER ID   IMAGE                  COMMAND           CREATED         STATUS         PORTS     NAMES
9a1f7f912f29   lahmanja/arch:latest   "/usr/bin/bash"   4 minutes ago   Up 4 minutes             archdev

```

Then commit the `CONTAINER ID` to a new image,

```shell
docker commit 9a1f7f912f29 lahmanja/arch:dirty
```

## Troubleshooting

```shell
Error response from daemon: Get "https://registry-1.docker.io/v2/": dial tcp: lookup registry-1.docker.io on [::1]:53: read udp [::1]:47503->[::1]:53: read: connection refused
```

Per https://github.com/docker/cli/issues/2618#issuecomment-863712429,
```shell
sudo snap disable docker
sudo snap enable docker
```
