package internal

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// Directory helpers

func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// File download helper

func DownloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
