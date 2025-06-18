package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ContainerHealth struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func main() {
	resp, err := http.Get("http://health.home/health")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var containers []ContainerHealth
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		panic(err)
	}

	for _, c := range containers {
		fmt.Printf("%s -> %s\n", c.Name, c.Status)
	}
}
