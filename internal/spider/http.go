package spider

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"web-spider/internal/metrics"
)

func DownloadHTML(url string, stats *metrics.CrawlerStats) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// HANDLE NON-OK RESPONSES
	if resp.StatusCode != http.StatusOK {
		errMsg := fmt.Sprintf("Non-OK HTTP status for %s: %d\n", url, resp.StatusCode)
		stats.MU.Lock()
		stats.HTTPErrors++
		stats.MU.Unlock()
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

	stats.MU.Lock()
	stats.HTMLPages++
	stats.MU.Unlock()

	return string(body), nil
}
