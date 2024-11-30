# work

`work` is a stupid simple time tracker.

## Usage

## Install

**pip:**

`work` is available as a [pypi package](https://pypi.org/project/gwork/).

```shell
pip install gwork
```

The recommended way to use this package is with an [`alias`](https://www.gnu.org/software/bash/manual/html_node/Aliases.html) using [`uvx`](https://docs.astral.sh/uv/guides/tools/),

```shell
echo "alias work='uvx --from=gwork work'" >> ~/.bashrc
source ~/.bashrc
```

**go:**

`work` can be installed by building from source,

```shell
go install github.com/jmelahman/work@latest
```

**github:**

Prebuilt packages are available from [Github Releases](https://github.com/jmelahman/work/releases).
