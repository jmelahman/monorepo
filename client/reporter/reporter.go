package reporter

import (
	"fmt"
	"log"
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
	defer func() {
		if err := r.writer.Flush(); err != nil {
			log.Printf("Error flushing writer: %v", err)
		}
	}()

	for _, task := range tasks {
		end := task.End
		if end.IsZero() {
			end = time.Now()
		}

		if _, err := fmt.Fprintf(
			r.writer,
			"%s - %s\t%s\t%s\t%s\n",
			task.Start.Format("15:04"),
			end.Format("15:04"),
			task.Classification,
			task.Description,
			r.FormatDuration(end.Sub(task.Start)),
		); err != nil {
			log.Printf("Error writing task row: %v", err)
		}
	}
}

func (r *Reporter) PrintReport(stats map[string]types.DayStats, weekTotal time.Duration) {
	defer func() {
		if err := r.writer.Flush(); err != nil {
			log.Printf("Error flushing writer: %v", err)
		}
	}()

	for day, dayStats := range stats {
		if _, err := fmt.Fprintf(r.writer, "%s\t%v\t(Total)\n", day, r.FormatDuration(dayStats.Total)); err != nil {
			log.Printf("Error writing report line: %v", err)
		}

		for classification, duration := range dayStats.ByClassification {
			if _, err := fmt.Fprintf(r.writer, "\t%v\t(%s)\n", r.FormatDuration(duration), classification); err != nil {
				log.Printf("Error writing classification line: %v", err)
			}
		}

		if _, err := fmt.Fprintln(r.writer, ""); err != nil {
			log.Printf("Error writing newline: %v", err)
		}
	}

	if _, err := fmt.Fprintf(r.writer, "\nWeek Total:\t%v\n", r.FormatDuration(weekTotal)); err != nil {
		log.Printf("Error writing week total: %v", err)
	}
}

func (r *Reporter) FormatDuration(duration time.Duration) string {
	return fmt.Sprintf("%dh %dmin", int(duration.Hours()), int(duration.Minutes())%60)
}
