package models

import (
	"time"
)

type Shift struct {
	id    int
	start time.Time
	end   time.Time
}

type Task struct {
	id          int
	description string
	start       time.Time
	end         time.Time
}
