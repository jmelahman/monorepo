module github.com/jmelahman/monorepo/dashboard

go 1.24.4

replace github.com/jmelahman/work => ../work

require (
	github.com/gdamore/tcell/v2 v2.9.0
	github.com/google/go-github/v57 v57.0.0
	github.com/jmelahman/docker-status v0.0.0-20250830075115-a46a185989e6
	github.com/jmelahman/work v1.0.12
	github.com/rivo/tview v0.42.0
	golang.org/x/oauth2 v0.31.0
)

require (
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mattn/go-sqlite3 v1.14.32 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/term v0.35.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)
