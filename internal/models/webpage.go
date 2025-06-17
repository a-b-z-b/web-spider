package models

type WebPage struct {
	Url   string   `bson:"url" json:"url"`
	Title string   `bson:"title" json:"title"`
	Text  string   `bson:"text" json:"text"`
	Links []string `bson:"links" json:"links"`
}
