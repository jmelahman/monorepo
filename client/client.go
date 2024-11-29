package client

import (
	"fmt"
	"time"

	"github.com/jmelahman/go-work/database"
	"github.com/jmelahman/go-work/database/models"
)

func HandleClockIn(dal *database.WorkDAL) (int, error) {
	latestShift, err := dal.GetLatestShift()
	if err != nil {
		return 1, err
	}

	if time.Until(latestShift.End) > 0 {
		return 0, nil
	}

	now := time.Now()
	err = dal.CreateShift(
		models.Shift{
			ID:    latestShift.ID + 1,
			Start: now,
			End:   now.Add(8 * time.Hour),
		},
	)
	if err != nil {
		return 1, err
	}
	return 0, nil
}

func HandleClockOut(dal *database.WorkDAL) (int, error) {
	latestShift, err := dal.GetLatestShift()
	if err != nil {
		return 1, err
	}
	if latestShift.ID != 0 && time.Until(latestShift.End) > 0 {
		dal.EndShift(latestShift.ID)
	}
	return 0, nil
}

func HandleList(dal *database.WorkDAL) (int, error) {
	tasks, err := dal.ListTasks(5)
	if err != nil {
		return 1, err
	}

	fmt.Printf("id, description, start, end\n")
	for _, t := range tasks {
		fmt.Printf("%d %v %v %v\n", t.ID, t.Description, t.Start.Format(time.DateTime), t.End.Format(time.DateTime))
	}
	return 0, nil
}

func HandleReport(dal *database.WorkDAL) (int, error) {
	//_, err := dal.GetShifts()
	//if err != nil {
	//	return 1, err
	//}
	return 1, fmt.Errorf("NOT IMPLEMENTED")
}

func HandleStatus(dal *database.WorkDAL, quiet bool) (int, error) {
	latestShift, err := dal.GetLatestShift()
	if err != nil {
		panic(err)
	}
	if latestShift.ID != 0 && time.Until(latestShift.End) > 0 {
		if !quiet {
			fmt.Printf("Hours left: %v\n", timeOnly(time.Until(latestShift.End)))
		}
		return 0, nil
	}
	if !quiet {
		fmt.Println("Not clocked in.")
		return 0, nil
	}
	return 1, nil
}

func HandleTask(dal *database.WorkDAL, description string) (int, error) {
	returncode, err := HandleClockIn(dal)
	if err != nil {
		return returncode, err
	}
	latestShift, err := dal.GetLatestShift()
	if err != nil {
		return 1, err
	}

	latestTask, err := dal.GetLatestTask()
	if err != nil {
		return 1, err
	}

	if time.Until(latestTask.End) > 0 {
		dal.EndTask(latestTask.ID)
	}
	err = dal.CreateTask(
		models.Task{
			ID:          latestTask.ID + 1,
			Description: description,
			Start:       time.Now(),
			End:         latestShift.End,
		})
	if err != nil {
		return 1, err
	}
	return 0, nil
}

func timeOnly(duration time.Duration) string {
	return fmt.Sprintf("%dh %dm", int(duration.Hours()), int(duration.Minutes())%60)
}
