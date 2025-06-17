package mongodb

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"os"
	"web-spider/internal/models"
	"web-spider/pkg/logger"
)

type DatabaseConnection struct {
	IsAccessible bool
	Uri          string
	Client       *mongo.Client
	Collection   *mongo.Collection
}

func (db *DatabaseConnection) Connect() {
	if db.IsAccessible {
		db.Uri = os.Getenv("MONGO_URI")
		dbClient, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(db.Uri))
		if err != nil {
			panic(err)
		}

		db.Client = dbClient
		db.Collection = db.Client.Database(os.Getenv("MONGO_DATABASE")).Collection(os.Getenv("MONGO_COLLECTION"))
	}
}

func (db *DatabaseConnection) Disconnect() {
	if db.IsAccessible {
		err := db.Client.Disconnect(context.TODO())
		if err != nil {
			panic(err)
		}
	}
}

func (db *DatabaseConnection) InsertWebPage(wp *models.WebPage) {
	if db.IsAccessible {
		if db.Collection == nil {
			logger.Error("mongo database collection `" + os.Getenv("MONGO_COLLECTION") + "` not found.")
			return
		}

		result, err := db.Collection.InsertOne(context.Background(), wp)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				logger.Error("Duplicate URL skipped: " + wp.Url)
			} else {
				logger.Error(fmt.Sprintf("Failed to insert page: %v\n", err))
			}
			return
		}

		logger.Success(fmt.Sprintf("Inserted URL with _id: %s\n", result.InsertedID))
	}
}
