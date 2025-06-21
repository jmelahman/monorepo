package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"text/tabwriter"
)

type ContainerHealth struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	State  string `json:"state"`
}

func GetContainerHealth(url *string) ([]ContainerHealth, error) {
	resp, err := http.Get(*url)
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %v", err)
	}
	defer resp.Body.Close()

	var containers []ContainerHealth
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	return containers, nil
}

func GetDockerStatus(url *string) (string, error) {
	containers, err := GetContainerHealth(url)
	if err != nil {
		return "", fmt.Errorf("failed to get container health: %v", err)
	}

	// Build the text content with colors using tabwriter
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)

	// Sort containers alphabetically by name
	sort.Slice(containers, func(i, j int) bool {
		return containers[i].Name < containers[j].Name
	})

	for _, c := range containers {
		// Determine status text and color
		statusText := c.Status
		if c.Status == "none" {
			statusText = c.State
		}

		var colorTag string
		var boldTag string
		switch statusText {
		case "healthy":
			colorTag = "green"
			boldTag = "::b"
		case "running":
			colorTag = "green"
		case "starting":
			colorTag = "yellow"
		case "paused":
			colorTag = "yellow"
		case "unhealthy":
			colorTag = "red"
			boldTag = "::b"
		case "exited":
			colorTag = "red"
		default:
			colorTag = "white"
		}

		fmt.Fprintf(w, "%s\t[%s%s]%s[-]\n", c.Name, colorTag, boldTag, statusText)
	}

	w.Flush()
	return buf.String(), nil
}
