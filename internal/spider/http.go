package spider

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func DownloadHTML(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// HANDLE NON-OK RESPONSES
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Non-OK HTTP status for %s: %d\n", url, resp.StatusCode)
		return "", errors.New(errMsg)
	}

	// HANDLE CONTENT TYPES AS SO IT'S ONLY a text/html CONTENT-TYPE
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		errMsg := fmt.Sprintf("Skipping non-HTML content at %s (Content-Type: %s)\n", url, contentType)
		return "", errors.New(errMsg)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
