package main

import "github.com/jmelahman/connections"

func main() {
	if err := connections.RunGame(); err != nil {
		panic(err)
	}
}
