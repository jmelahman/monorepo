package api

type ContainerHealth struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	State  string `json:"state"`
}
