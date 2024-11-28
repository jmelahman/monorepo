package models

import (
	"time"
)

type Shift struct {
	ID    int       `json:"id"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type Task struct {
	ID          int       `json:"id"`
	Description string    `json:"description"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
}
