package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"runtime"
	"strconv"
	"time"
	"web-spider/internal/database/mongodb"
	"web-spider/internal/filter"
	"web-spider/internal/frontier"
	"web-spider/internal/metrics"
	"web-spider/internal/parser"
	"web-spider/internal/spider"
	"web-spider/pkg/logger"
)

func main() {
	runtime.GOMAXPROCS(8)
	fmt.Println("GOMAXPROCS:", runtime.GOMAXPROCS(0))

	env := flag.String("env", "prod", "Application environment.")
	workers := flag.Int("workers", 16, "Number of concurrent workers.")
	threshold := flag.Int("threshold", 100, "Maximum number of pages to crawl.")

	flag.Parse()

	// DATABASE SETUP
	dbAccess := true
	var loading error
	if *env == "test" {
		loading = godotenv.Load(".env.test")
	} else {
		loading = godotenv.Load(".env")
	}
	if loading != nil {
		logger.Error("Error loading .env file. Preventing access to crawler dataset.")
		dbAccess = false
	}

	dbConnection := mongodb.DatabaseConnection{
		IsAccessible: dbAccess,
		Uri:          "",
		Client:       nil,
		Collection:   nil,
	}

	dbConnection.Connect()
	defer dbConnection.Disconnect()

	textIdx := mongo.IndexModel{
		Keys:    bson.D{{Key: "text", Value: "text"}},
		Options: options.Index().SetName("TextIndex"),
	}
	urlIdx := mongo.IndexModel{
		Keys:    bson.D{{Key: "url", Value: 1}},
		Options: options.Index().SetName("UrlIndex"),
	}
	if dbConnection.IsAccessible {
		_, err := dbConnection.Collection.Indexes().CreateOne(context.TODO(), textIdx)
		if err != nil {
			fmt.Println(err)
		}
		_, err = dbConnection.Collection.Indexes().CreateOne(context.TODO(), urlIdx)
		if err != nil {
			fmt.Println(err)
		}
	} else {
		log.Fatal("Database is not accessible. Halting crawler.")
	}

	// STRUCTURES SETUP
	jobs := make(chan string, 100)
	done := make(chan bool)
	urlFrontier := frontier.Frontier{Items: make([]string, 0, 1000)}
	crawlerSet := filter.UrlSet{Set: make(map[uint64]bool, 1000)}
	seeds := []string{
		"https://news.ycombinator.com",
		"https://wikipedia.org",
	}

	// STATS SETUP
	crawlerStats := metrics.NewCrawlerStats()
	doneMetrics := make(chan bool)
	ticker := time.NewTicker(time.Second)

	go func() {
		for {
			select {
			case <-doneMetrics:
				return
			case t := <-ticker.C:
				crawlerStats.CrawlingPerMinuteRate(&urlFrontier, &crawlerSet, t)
			}
		}
	}()

	// SEEDING CRAWLER
	for _, seed := range seeds {
		nUrl, err := filter.NormalizeUrl(seed)
		if err != nil {
			log.Fatal(err)
		}

		urlFrontier.Enqueue(nUrl)
		crawlerSet.Add(nUrl)

		crawlerStats.TotalSeen++
		crawlerStats.UniqueEnqueued++

		jobs <- nUrl
	}

	// SPIN-UP WORKER GOROUTINES
	for i := 0; i < *workers; i++ {
		go processUrl(i, *threshold, jobs, done, &dbConnection, &urlFrontier, &crawlerSet, crawlerStats)
	}

	// GOROUTINE FEEDER (dispatcher goroutine)
	go func() {
		for {
			if urlFrontier.TotalProcessedUrls() >= *threshold {
				logger.Warn("🛑 Threshold reached.")
				close(jobs)
				return
			}

			urlItem, ok := urlFrontier.TryDequeue()
			if !ok {
				logger.Info("Unsuccessful dequeue! Sleeping...")
				time.Sleep(100 * time.Millisecond)
				continue
			}

			logger.Info("Successful dequeue!")
			jobs <- urlItem
		}
	}()

	// Wait for all goroutines to finish.
	for i := 0; i < *workers; i++ {
		<-done
	}

	logger.Info(fmt.Sprintf("\n\nTotal Procesed: `%d`\n\n", urlFrontier.TotalProcessedUrls()))
	ticker.Stop()
	doneMetrics <- true

	logger.Info(fmt.Sprintf("Raw Stats → TotalSeen: %d, UniqueEnqueued: %d, DBInserted: %d, DBInsertAttempts: %d, FailedInserts: %d, HTMLPages: %d, EmptyPages: %d, SkippedDuplicates: %d, HTTPErrors: %d",
		crawlerStats.TotalSeen,
		crawlerStats.UniqueEnqueued,
		crawlerStats.DBInserted,
		crawlerStats.DBInsertAttempts,
		crawlerStats.FailedInserts,
		crawlerStats.HTMLPages,
		crawlerStats.EmptyPages,
		crawlerStats.SkippedDuplicates,
		crawlerStats.HTTPErrors,
	))
	crawlerStats.PrintTimingStats()
	crawlerStats.PrintGeneralStats()
	fmt.Printf("\n\nProgram Finished. It took: %v\n\n", time.Since(crawlerStats.StartedAt))
}

func processUrl(id, threshold int, jobs chan string, done chan bool, dbConnection *mongodb.DatabaseConnection, urlFrontier *frontier.Frontier, crawlerSet *filter.UrlSet, stats *metrics.CrawlerStats) {
	defer logger.Info("Goroutine " + strconv.Itoa(id) + " finished.")
	for url := range jobs {
		nUrl, err := filter.NormalizeUrl(url)
		if err != nil {
			fmt.Println(err)
			continue
		}

		fmt.Println("Crawling: `" + nUrl + "` - Crawling count: " + strconv.Itoa(crawlerSet.Size()))

		rawMarkup, err := spider.DownloadHTML(nUrl, stats)
		if err != nil {
			fmt.Println(err)
			continue
		}

		wp, err := parser.ParseHTML(nUrl, rawMarkup)
		if err != nil {
			fmt.Println(err)
			continue
		}

		if wp.Title == "" {
			logger.Warn(fmt.Sprintf("Skipping page without a title: %s\n", wp.Title))
			continue
		}
		if wp.Text == "" && len(wp.Links) == 0 {
			logger.Warn(fmt.Sprintf("Skipping empty page: %s\n", wp.Url))
			stats.MU.Lock()
			stats.EmptyPages++
			stats.MU.Unlock()
			continue
		}

		resultOp := dbConnection.InsertWebPage(wp, stats)
		if resultOp {
			crawlerSet.Add(nUrl)
		}

		for _, link := range wp.Links {
			newUrl, nErr := filter.NormalizeUrl(link)
			if nErr != nil {
				fmt.Println(nErr)
				continue
			}

			if !crawlerSet.Contains(newUrl) {
				stats.MU.Lock()
				val := stats.UniqueEnqueued
				stats.MU.Unlock()
				if val >= threshold {
					break
				}

				urlFrontier.Enqueue(newUrl)

				stats.MU.Lock()
				stats.TotalSeen++
				stats.UniqueEnqueued++
				stats.MU.Unlock()
			} else {
				logger.Info(fmt.Sprintf("Skipping: `%s` is already discovered.", newUrl))
				stats.MU.Lock()
				stats.SkippedDuplicates++
				stats.MU.Unlock()
			}
		}
	}
	done <- true
}
