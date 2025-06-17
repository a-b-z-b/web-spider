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
	"web-spider/internal/database/mongodb"
	"web-spider/internal/filter"
	"web-spider/internal/frontier"
	"web-spider/internal/parser"
	"web-spider/internal/spider"
	"web-spider/pkg/logger"
)

func main() {
	threshold := flag.Int("threshold", 100, "Maximum number of pages to crawl")

	flag.Parse()

	// DATABASE SETUP
	dbAccess := true
	if godotenv.Load() != nil {
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

	for _, seed := range seeds {
		nUrl, err := filter.NormalizeUrl(seed)
		if err != nil {
			log.Fatal(err)
		}

		urlFrontier.Enqueue(nUrl)
		crawlerSet.Add(nUrl)
	}

	// Kick-start the crawling flow...
	for urlFrontier.Size() > 0 && urlFrontier.TotalProcessedUrls() <= *threshold {
		urlItem := urlFrontier.Dequeue()

		nUrl, err := filter.NormalizeUrl(urlItem)
		if err != nil {
			fmt.Println(err)
			continue
		}

		log.Printf("🔎 Normalize check: raw=%s normalized=%s", urlItem, nUrl)

		fmt.Println("Crawling: `" + nUrl + "` - Crawling count: " + strconv.Itoa(crawlerSet.Size()))

		rawMarkup, err := spider.DownloadHTML(nUrl)
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
			continue
		}

		dbConnection.InsertWebPage(wp)
		crawlerSet.Add(nUrl)

		for _, link := range wp.Links {
			url, err := filter.NormalizeUrl(link)
			if err != nil {
				fmt.Println(err)
				continue
			}

			if !crawlerSet.Contains(url) {
				urlFrontier.Enqueue(url)
			} else {
				logger.Info(fmt.Sprintf("Skipping: `%s` is already discovered.", url))
			}
		}
	}

	logger.Info(fmt.Sprintf("\n\nTotal Procesed: `%d`\n\n", urlFrontier.TotalProcessed))

}
