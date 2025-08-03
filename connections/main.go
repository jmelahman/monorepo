package main

import "github.com/jmelahman/connections/game"

func main() {
	if err := game.Run(); err != nil {
		panic(err)
	}
}
