package client

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jmelahman/work/database"
	"github.com/jmelahman/work/database/models"
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
			End:   time.Time{},
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

func HandleList(dal *database.WorkDAL, limit int) (int, error) {
	tasks, err := dal.ListTasks(0, 1)
	if err != nil {
		return 1, err
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	for _, t := range tasks {
		fmt.Fprintf(
			w,
			"%s - %s\t%s\t%s\n",
			t.Start.Format("15:04"),
			t.End.Format("15:04"),
			t.Description,
			timeOnly(t.End.Sub(t.Start)),
		)
	}
	w.Flush()
	return 0, nil
}

func HandleReport(dal *database.WorkDAL) (int, error) {
	shifts, err := dal.ListShifts(0, 5)
	if err != nil {
		return 1, err
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 0, '\t', 0)
	for _, s := range shifts {
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\n",
			s.Start.Format("01/02 15:04"),
			s.End.Format("01/02 15:04"),
			timeOnly(s.End.Sub(s.Start)),
		)
	}
	w.Flush()
	return 0, nil
}

func HandleStatus(dal *database.WorkDAL, quiet bool) (int, error) {
	latestShift, err := dal.GetLatestShift()
	if err != nil {
		panic(err)
	}

	expectedEnd := latestShift.Start.Add(8 * time.Hour)
	remainingTime := expectedEnd.Sub(latestShift.End)
	if latestShift.ID != 0 && remainingTime > 0 {
		if !quiet {
			latestTask, err := dal.GetLatestTask()
			if err != nil {
				return 1, err
			}
			fmt.Printf("Hours left:   %s\n", timeOnly(remainingTime))
			fmt.Printf("Current task: \"%s\"\n", latestTask.Description)
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
			End:         time.Time{},
		})
	if err != nil {
		return 1, err
	}
	return 0, nil
}

func timeOnly(duration time.Duration) string {
	return fmt.Sprintf("%dh %dmin", int(duration.Hours()), int(duration.Minutes())%60)
}
