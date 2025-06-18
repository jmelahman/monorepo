package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type ContainerHealth struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func main() {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		containers, err := cli.ContainerList(context.Background(), container.ListOptions{All: true})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var healthStatuses []ContainerHealth
		for _, c := range containers {
			info, err := cli.ContainerInspect(context.Background(), c.ID)
			if err != nil || info.State == nil {
				continue
			}
			status := "none"
			if info.State.Health != nil {
				status = info.State.Health.Status
			}
			healthStatuses = append(healthStatuses, ContainerHealth{
				Name:   c.Names[0],
				Status: status,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(healthStatuses)
	})

	log.Println("Listening on :9090...")
	log.Fatal(http.ListenAndServe(":9090", nil))
}
