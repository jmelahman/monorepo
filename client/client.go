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

	if latestShift.End.IsZero() && !latestShift.Start.IsZero() {
		return 0, nil
	}

	err = dal.CreateShift(
		models.Shift{
			ID:    latestShift.ID + 1,
			Start: time.Now(),
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
	if latestShift.End.IsZero() {
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
		var end time.Time
		if t.End.IsZero() {
			end = time.Now()
		} else {
			end = t.End
		}
		fmt.Fprintf(
			w,
			"%s - %s\t%s\t%s\t%s\n",
			t.Start.Format("15:04"),
			end.Format("15:04"),
			t.Classification,
			t.Description,
			timeOnly(end.Sub(t.Start)),
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
		var end time.Time
		if s.End.IsZero() {
			end = time.Now()
		} else {
			end = s.End
		}
		fmt.Fprintf(
			w,
			"%d\t%s - %s\t%s\n",
			s.ID,
			s.Start.Format("01/02 15:04"),
			end.Format("01/02 15:04"),
			timeOnly(end.Sub(s.Start)),
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

	if latestShift.ID != 0 && latestShift.End.IsZero() {
		if !quiet {
			latestTask, err := dal.GetLatestTask()
			if err != nil {
				return 1, err
			}
			remainingTime := time.Until(latestShift.Start.Add(8 * time.Hour))
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

func HandleTask(
	dal *database.WorkDAL,
	classification models.TaskClassification,
	description string,
) (int, error) {
	returncode, err := HandleClockIn(dal)
	if err != nil {
		return returncode, err
	}

	latestTask, err := dal.GetLatestTask()
	if err != nil {
		return 1, err
	}

	if latestTask.End.IsZero() {
		dal.EndTask(latestTask.ID)
	}
	err = dal.CreateTask(
		models.Task{
			ID:             latestTask.ID + 1,
			Description:    description,
			Classification: classification,
			Start:          time.Now(),
			End:            time.Time{},
		})
	if err != nil {
		return 1, err
	}
	return 0, nil
}

func timeOnly(duration time.Duration) string {
	return fmt.Sprintf("%dh %dmin", int(duration.Hours()), int(duration.Minutes())%60)
}
