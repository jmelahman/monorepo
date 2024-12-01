# work

[![Deploy Status](https://github.com/jmelahman/work/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/work/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/jmelahman/work.svg)](https://pkg.go.dev/github.com/jmelahman/work)
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

$ work stop
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

### Shutdown  service

Optionally, install a `systemd` user service which stops any running tasks on shutdown.

To install and enable the service,

```shell
work install
```

### Autocomplete

`work` provides shell autocomplete out-of-the-box.
To enable autocomplete,


```shell
work install-completion
```

## Install

**pip:**

`work` is available as a [pypi package](https://pypi.org/project/gwork/).

```shell
pip install gwork
```

**go:**

```shell
go install github.com/jmelahman/work@latest
```

**github:**

Prebuilt packages are available from [Github Releases](https://github.com/jmelahman/work/releases).
