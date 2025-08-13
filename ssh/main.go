package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/terminfo"
	"github.com/gliderlabs/ssh"
	"github.com/jmelahman/connections/game"
	"github.com/spf13/cobra"
)

var (
	port    string
	keyFile string
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "ssh-game",
		Short: "SSH game server",
		Run: func(cmd *cobra.Command, args []string) {
			serve()
		},
	}

	rootCmd.Flags().StringVar(&port, "port", "2222", "Port to listen on")
	rootCmd.Flags().StringVar(&keyFile, "key-file", "", "Path to SSH host key file")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func serve() {
	ssh.Handle(func(s ssh.Session) {
		screen, err := NewSessionScreen(s)
		if err != nil {
			panic(err)
		}

		if err := game.RunWithScreen(screen); err != nil {
			panic(err)
		}
	})

	log.Printf("Starting SSH server on :%s", port)

	// Use provided key file or default to ~/.ssh/id_rsa
	hostKeyFile := keyFile
	if hostKeyFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		hostKeyFile = filepath.Join(home, ".ssh", "id_rsa")
	}

	log.Fatal(ssh.ListenAndServe(":"+port, nil,
		ssh.HostKeyFile(hostKeyFile),
	))
}

func NewSessionScreen(s ssh.Session) (tcell.Screen, error) {
	pi, ch, ok := s.Pty()
	if !ok {
		return nil, errors.New("no pty requested")
	}
	ti, err := terminfo.LookupTerminfo(pi.Term)
	if err != nil {
		return nil, err
	}
	screen, err := tcell.NewTerminfoScreenFromTtyTerminfo(&tty{
		Session: s,
		size:    pi.Window,
		ch:      ch,
	}, ti)
	if err != nil {
		return nil, err
	}
	return screen, nil
}

type tty struct {
	ssh.Session
	size     ssh.Window
	ch       <-chan ssh.Window
	resizecb func()
	mu       sync.Mutex
}

func (t *tty) Start() error {
	go func() {
		for win := range t.ch {
			t.size = win
			t.notifyResize()
		}
	}()
	return nil
}

func (t *tty) Stop() error {
	return nil
}

func (t *tty) Drain() error {
	return nil
}

func (t *tty) WindowSize() (tcell.WindowSize, error) {
	return tcell.WindowSize{Width: t.size.Width, Height: t.size.Height}, nil
}

func (t *tty) NotifyResize(cb func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.resizecb = cb
}

func (t *tty) notifyResize() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.resizecb != nil {
		t.resizecb()
	}
}
