# work

[![Test status](https://github.com/jmelahman/work/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/work/actions)
[![Deploy Status](https://github.com/jmelahman/work/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/work/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/jmelahman/work.svg)](https://pkg.go.dev/github.com/jmelahman/work)
[![Arch User Repsoitory](https://img.shields.io/aur/version/work)](https://aur.archlinux.org/packages/work)
[![PyPI](https://img.shields.io/pypi/v/work-bin.svg)]()
[![Go Report Card](https://goreportcard.com/badge/github.com/jmelahman/work)](https://goreportcard.com/report/github.com/jmelahman/work)

`work` is a stupid simple time tracker.

A typical `work`-day might look like,

```shell
$ work task "First task of the day"

$ work task "Second task of the day" --chore

$ work list
08:29 - 10:20   Chore   Second task of the day  2h 51min
08:19 - 08:29   Work    First task of the day   0h 10min

$ work status
Current task: "Second task of the day"
Type: Chore
Duration: 2h 51min

$ work stop

$ work report
2024-12-01      3h 01min (Total)
                2h 51min (Chore)
                0h 10min (Work)


Week Total:     3h 01min
```

## Usage

To begin a task,

```shell
work task "Starting my first task"
```

Tasks have 1 of 4 possible classifications:

- `Chore`
- `Break`
- `Toil`
- `Work`

The default is `Work` and the others are enabled by their respective flag.
For example, to create a `Break` task,

```shell
work task --break "Going for lunch"
```

The previous task will end when a new task begins.
Similarly, a task can be stopped explicitly with,

```shell
work stop
```

`work status`, `work list`, and `work report` are available to analyze current and previous tasks.

### Shutdown and Notification services

Optionally, install [`systemd` user services](https://wiki.archlinux.org/title/Systemd/User) which notify you when you're not tracking any tasks and stop any running tasks on system shutdown.

To install and enable the services,

```shell
work install
```

They can be disabled with,

```
work uninstall
```

### Autocomplete

`work` provides autocomplete for `bash`, `fish`, `powershell` and `zsh` shells.
For example, to enable autocomplete for the `bash` shell,

```shell
work completion bash | sudo tee /etc/bash_completion.d/work > /dev/null
```

_Note: this does require the [bash-completion](https://github.com/scop/bash-completion/) package is installed._

For more information, see `work completion <shell> --help` for your respective `<shell>`.

## Install

**AUR:**

`work` is available from the [Arch User Repository](https://aur.archlinux.org/packages/work).

```shell
yay -S work
```
**pip:**

`work` is available as a [pypi package](https://pypi.org/project/work-bin/).

```shell
pip install work-bin
```

**go:**

```shell
go install github.com/jmelahman/work@latest
```

**github:**

Prebuilt packages are available from [Github Releases](https://github.com/jmelahman/work/releases).
