# Physics in C++

## Requirements

### Bazel
Bazel comes pre-installed my personal development docker image.

It can be installed locally by running,

```shell
# pacman -S bazel
```

## Source

```shell
$ bazel run //src:all
```

## Testing

To run unit tests, run,

```shell
bazel test //test:all
```
