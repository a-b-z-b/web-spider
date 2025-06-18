package metrics

import (
	"fmt"
	"time"
	"web-spider/internal/filter"
	"web-spider/internal/frontier"
	"web-spider/pkg/logger"
	"web-spider/pkg/utils"
)

type CrawlerStats struct {
	TotalSeen             int
	UniqueEnqueued        int
	DBInsertAttempts      int
	DBInserted            int
	FailedInserts         int
	HTMLPages             int
	EmptyPages            int
	SkippedDuplicates     int
	HTTPErrors            int
	PagesPerMinute        string
	CrawledRatioPerMinute string
	StartedAt             time.Time
	EndedAt               time.Time
}

func NewCrawlerStats() *CrawlerStats {
	return &CrawlerStats{StartedAt: time.Now()}
}

func (c *CrawlerStats) EndCrawl() {
	c.EndedAt = time.Now()
}

func (c *CrawlerStats) URLUniquenessRatio() float64 {
	return utils.SafeDivide(c.UniqueEnqueued, c.TotalSeen)
}

func (c *CrawlerStats) InsertSuccessRate() float64 {
	return utils.SafeDivide(c.DBInserted, c.UniqueEnqueued)
}

func (c *CrawlerStats) InsertFailureRate() float64 {
	return utils.SafeDivide(c.FailedInserts, c.DBInsertAttempts)
}

func (c *CrawlerStats) HTMLPagesRatio() float64 {
	return utils.SafeDivide(c.HTMLPages, c.TotalSeen)
}

func (c *CrawlerStats) EmptyPagesRate() float64 {
	return utils.SafeDivide(c.EmptyPages, c.HTMLPages)
}

func (c *CrawlerStats) HTTPErrorRate() float64 {
	return utils.SafeDivide(c.HTTPErrors, c.TotalSeen)
}

func (c *CrawlerStats) DuplicatesSkipRate() float64 {
	return utils.SafeDivide(c.SkippedDuplicates, c.TotalSeen)
}

func (c *CrawlerStats) StorageYield() float64 {
	return utils.SafeDivide(c.DBInserted, c.TotalSeen)
}

func (c *CrawlerStats) CrawlingPerMinuteRate(q *frontier.Frontier, s *filter.UrlSet, t time.Time) {
	c.PagesPerMinute += fmt.Sprintf("%f %d\n", t.Sub(c.StartedAt).Minutes(), s.Size())
	c.CrawledRatioPerMinute += fmt.Sprintf("%f %f\n", t.Sub(c.StartedAt).Minutes(), utils.SafeDivide(s.Size(), q.Size()))
}

func (c *CrawlerStats) PrintGeneralStats() {
	logger.Info("\n------------------BEGIN CRAWLING GENERAL STATS PRINTING:")
	fmt.Printf("URL Uniqueness Ratio: %.2f\n", c.URLUniquenessRatio())
	fmt.Printf("Insertion Success Rate: %.2f\n", c.InsertSuccessRate())
	fmt.Printf("Insert Failure Rate: %.2f\n", c.InsertFailureRate())
	fmt.Printf("HTML Page Ratio: %.2f\n", c.HTMLPagesRatio())
	fmt.Printf("Empty Page Rate: %.2f\n", c.EmptyPagesRate())
	fmt.Printf("Duplicate Skip Rate: %.2f\n", c.DuplicatesSkipRate())
	fmt.Printf("Error Rate (HTTP): %.2f\n", c.HTTPErrorRate())
	fmt.Printf("Storage Yield: %.2f\n", c.StorageYield())
	logger.Info("\n------------------END CRAWLING GENERAL STATS PRINTING.")
}

func (c *CrawlerStats) PrintTimingStats() {
	logger.Info("\n------------------BEGIN CRAWLING TIMING STATS PRINTING:")
	fmt.Println("Pages crawled per minute:")
	fmt.Println(c.PagesPerMinute)
	fmt.Println("Crawl to Queued Ratio per minute:")
	fmt.Println(c.CrawledRatioPerMinute)
	logger.Info("\n------------------END CRAWLING TIMING STATS PRINTING.")
}
