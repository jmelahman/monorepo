package reporter

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/jmelahman/work/database/types"
)

// Reporter handles report generation
type Reporter struct {
	writer *tabwriter.Writer
}

// NewReporter creates a new Reporter instance
func NewReporter() *Reporter {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 5, 8, 1, '\t', 0)
	return &Reporter{
		writer: w,
	}
}

func (r *Reporter) PrintTaskRows(tasks []types.Task) {
	defer r.writer.Flush()

	for _, task := range tasks {
		end := task.End
		if end.IsZero() {
			end = time.Now()
		}

		fmt.Fprintf(
			r.writer,
			"%s - %s\t%s\t%s\t%s\n",
			task.Start.Format("15:04"),
			end.Format("15:04"),
			task.Classification,
			task.Description,
			r.FormatDuration(end.Sub(task.Start)),
		)
	}
}

func (r *Reporter) PrintReport(stats map[string]types.DayStats, weekTotal time.Duration) {
	defer r.writer.Flush()

	for day, dayStats := range stats {
		fmt.Fprintf(r.writer, "%s\t%v\t(Total)\n", day, r.FormatDuration(dayStats.Total))

		for classification, duration := range dayStats.ByClassification {
			fmt.Fprintf(r.writer, "\t%v\t(%s)\n", r.FormatDuration(duration), classification)
		}
		fmt.Fprintln(r.writer, "")
	}

	fmt.Fprintf(r.writer, "\nWeek Total:\t%v\n", r.FormatDuration(weekTotal))
}

func (r *Reporter) FormatDuration(duration time.Duration) string {
	return fmt.Sprintf("%dh %dmin", int(duration.Hours()), int(duration.Minutes())%60)
}
