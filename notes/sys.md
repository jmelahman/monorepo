# System Administration

## Pruning

### journalctl

Garbage collect `journalctl` logs,

```shell
sudo journalctl --vacuum-time=2d
```

### snap

Remove old `snap` package revisions,

/* From https://superuser.com/a/1356803 */

```shell
snap list --all | while read snapname ver rev trk pub notes; do if [[ $notes = *disabled* ]]; then sudo snap remove "$snapname" --revision="$rev"; fi; done
```

Only retain a small number of revisions in the future (default is `3`),

/* From https://snapcraft.io/docs/keeping-snaps-up-to-date#heading--refresh-retain */

```shell
sudo snap set system refresh.timer=2
```

### docker

```shell
docker system prune -af
```
