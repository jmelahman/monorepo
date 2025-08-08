module github.com/jmelahman/monorepo/dashboard

go 1.24.4

replace github.com/jmelahman/work => ../work

require (
	github.com/gdamore/tcell/v2 v2.8.1
	github.com/google/go-github/v57 v57.0.0
	github.com/jmelahman/docker-status v0.0.0-20250621064045-62a95a1e66e1
	github.com/jmelahman/work v0.0.0-00010101000000-000000000000
	github.com/rivo/tview v0.0.0-20250625164341-a4a78f1e05cb
	golang.org/x/oauth2 v0.30.0
)

require (
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mattn/go-sqlite3 v1.14.28 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/term v0.34.0 // indirect
	golang.org/x/text v0.28.0 // indirect
)
