# work

`work` is a stupid simple time tracker.

## Usage

## Install

**pip:**

`work` is also available as a [pypi package](https://pypi.org/project/gwork/).

```shell
pip install gwork
```

or executed directly via [`uvx`](https://docs.astral.sh/uv/guides/tools/),

```shell
uvx --from gwork work
```

The recommended way to use this package is with an `alias` using `uvx`,

```shell
echo "alias work='uvx --from=gwork work'" >> ~/.bashrc
source ~/.bashrc
```

**go:**

You may also build from source,

```shell
go install github.com/jmelahman/work@latest
```
