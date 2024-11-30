# work

`work` is a stupid simple time tracker.

## Usage

`work` tracks time in shifts and tasks.
A shift may contain many tasks and a task may span multiple shifts.

To start and stop a shift,

```shell
work clock-in
```

```shell
work clock-out
```

Shifts will begin automatically when starting a task and ending a shift will end tasks.

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

`work status`, `work list`, and `work report` are available to reflect on previous shifts and
tasks.

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
