package main

import (
	"log"

	"github.com/gliderlabs/ssh"
	"github.com/jmelahman/connections/game"
)

func main() {
	ssh.Handle(func(s ssh.Session) {
		game.Run(s, s, s)
	})

	log.Println("Starting SSH server on :22")
	log.Fatal(ssh.ListenAndServe(":22", nil,
		ssh.HostKeyFile("/home/jamison/.ssh/id_rsa"),
	))
}
