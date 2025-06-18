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
	env := flag.String("env", "prod", "Application environment.")
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
	urlFrontier := frontier.Frontier{Items: make([]string, 0, 1000)}
	crawlerSet := filter.UrlSet{Set: make(map[uint64]bool, 1000)}
	seeds := []string{
		"https://news.ycombinator.com",
		"https://wikipedia.org",
	}

	// STATS SETUP
	crawlerStats := metrics.NewCrawlerStats()
	done := make(chan bool)
	ticker := time.NewTicker(10 * time.Second)

	go func() {
		for {
			select {
			case <-done:
				return
			case t := <-ticker.C:
				crawlerStats.CrawlingPerMinuteRate(&urlFrontier, &crawlerSet, t)
			}
		}
	}()

	for _, seed := range seeds {
		nUrl, err := filter.NormalizeUrl(seed)
		if err != nil {
			log.Fatal(err)
		}

		urlFrontier.Enqueue(nUrl)
		crawlerSet.Add(nUrl)

		crawlerStats.TotalSeen++
		crawlerStats.UniqueEnqueued++
	}

	// Kick-start the crawling flow...
	for urlFrontier.Size() > 0 && urlFrontier.TotalProcessedUrls() < *threshold {
		urlItem := urlFrontier.Dequeue()

		nUrl, err := filter.NormalizeUrl(urlItem)
		if err != nil {
			fmt.Println(err)
			continue
		}

		fmt.Println("Crawling: `" + nUrl + "` - Crawling count: " + strconv.Itoa(crawlerSet.Size()))

		rawMarkup, err := spider.DownloadHTML(nUrl, crawlerStats)
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
			crawlerStats.EmptyPages++
			continue
		}

		resultOp := dbConnection.InsertWebPage(wp, crawlerStats)
		if resultOp {
			crawlerSet.Add(nUrl)
		}

		for _, link := range wp.Links {
			url, err := filter.NormalizeUrl(link)
			if err != nil {
				fmt.Println(err)
				continue
			}

			if !crawlerSet.Contains(url) {
				if crawlerStats.UniqueEnqueued >= *threshold {
					break
				}
				urlFrontier.Enqueue(url)

				crawlerStats.TotalSeen++
				crawlerStats.UniqueEnqueued++
			} else {
				logger.Info(fmt.Sprintf("Skipping: `%s` is already discovered.", url))
				crawlerStats.SkippedDuplicates++
			}
		}
	}

	logger.Info(fmt.Sprintf("\n\nTotal Procesed: `%d`\n\n", urlFrontier.TotalProcessedUrls()))
	ticker.Stop()
	done <- true

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
}
