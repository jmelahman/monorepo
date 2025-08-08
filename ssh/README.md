# Connections Over SSH

[![Test status](https://github.com/jmelahman/connections-ssh/actions/workflows/test.yml/badge.svg)](https://github.com/jmelahman/connections-ssh/actions)
[![Deploy Status](https://github.com/jmelahman/connections-ssh/actions/workflows/release.yml/badge.svg)](https://github.com/jmelahman/connections-ssh/actions)
[![Go Reference](https://pkg.go.dev/badge/github.com/jmelahman/connections-ssh.svg)](https://pkg.go.dev/github.com/jmelahman/connections-ssh)
[![Go Report Card](https://goreportcard.com/badge/github.com/jmelahman/connections-ssh)](https://goreportcard.com/report/github.com/jmelahman/connections-ssh)

This serves the [NYT Connections TUI](https://github.com/jmelahman/connections) over SSH.

## Running

In one terminal, start the server,

```shell
$ go run .
2025/08/07 22:55:16 Starting SSH server on :2222
```

In a separate terminal,

```shell
$ ssh -p 2222 localhost
```

This assumes the server has an [SSH key](https://wiki.archlinux.org/title/SSH_keys) at [~/.ssh/id_rsa](https://github.com/jmelahman/connections-ssh/blob/12b9ba7d3ec6059a349d23ea85e7b948b16517a1/main.go#L32).

Moreover, if running on port `22` is desired, you'll like need elevated privileges (not recommended),

```shell
$ go build .
$ sudo ./connections-ssh
2025/08/07 22:55:16 Starting SSH server on :22
```

## Install

**go:**

```shell
go install github.com/jmelahman/connections-ssh@latest
```

**github:**

Prebuilt packages are available from [Github Releases](https://github.com/jmelahman/connections-ssh/releases).
