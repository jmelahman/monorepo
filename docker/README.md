# Docker

## Troubleshooting

```shell
Error response from daemon: Get "https://registry-1.docker.io/v2/": dial tcp: lookup registry-1.docker.io on [::1]:53: read udp [::1]:47503->[::1]:53: read: connection refused
```

Per https://github.com/docker/cli/issues/2618#issuecomment-863712429,
```shell
sudo snap disable docker
sudo snap enable docker
```
