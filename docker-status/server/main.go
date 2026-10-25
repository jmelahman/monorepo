package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/jmelahman/docker-status/api"
	"github.com/moby/moby/client"
)

func main() {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		containers, err := cli.ContainerList(context.Background(), client.ContainerListOptions{All: true})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var healthStatuses []api.ContainerHealth
		for _, c := range containers.Items {
			res, err := cli.ContainerInspect(context.Background(), c.ID, client.ContainerInspectOptions{})
			if err != nil || res.Container.State == nil {
				continue
			}
			status := "none"
			if h := res.Container.State.Health; h != nil {
				status = string(h.Status)
			}
			healthStatuses = append(healthStatuses, api.ContainerHealth{
				Name:   c.Names[0],
				Status: status,
				State:  string(c.State),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(healthStatuses)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	log.Println("Listening on :9090...")
	log.Fatal(http.ListenAndServe(":9090", nil))
}
