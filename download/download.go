package download

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
)

type ProgressReader struct {
	Reader         io.Reader
	TotalSize      int64
	DownloadedSize int64
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.DownloadedSize += int64(n)

	if pr.TotalSize > 0 {
		percent := float64(pr.DownloadedSize) / float64(pr.TotalSize) * 100
		fmt.Printf("\rDownloading.. %.0f%%", percent)
	} else {
		fmt.Printf("\rDownloading... %.0f MB", float64(pr.DownloadedSize)/(1024*1024))
	}

	return n, err
}

func FileWithProgress(url, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	contentLength, _ := strconv.Atoi(resp.Header.Get("Content-Length"))

	progressReader := &ProgressReader{
		Reader:         resp.Body,
		TotalSize:      int64(contentLength),
		DownloadedSize: 0,
	}

	_, err = io.Copy(out, progressReader)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}
