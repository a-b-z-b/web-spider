package parser

import (
	"fmt"
	"golang.org/x/net/html"
	"strings"
	"web-spider/internal/models"
)

func ParseHTML(url, rawHTML string) (*models.WebPage, error) {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return nil, err
	}

	title := extractTitle(doc)
	text := extractText(doc)
	links := extractLinks(doc)

	wp := &models.WebPage{
		Url:   url,
		Title: title,
		Text:  text,
		Links: links,
	}

	return wp, nil
}

func extractTitle(doc *html.Node) string {
	var title string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
			title = n.FirstChild.Data
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return title
}

func extractText(doc *html.Node) string {
	var text string
	var f func(*html.Node)

	tokensLimit := 0
	f = func(n *html.Node) {
		if n.Type == html.TextNode && !isDescendantOfSkippableTag(n) {
			words := strings.Fields(n.Data)
			for _, word := range words {
				if tokensLimit >= 500 {
					break
				}

				text = fmt.Sprintf("%s %s", text, word)
				tokensLimit++
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return strings.TrimSpace(text)
}

func extractLinks(doc *html.Node) []string {
	var links []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" && strings.HasPrefix(attr.Val, "http") {
					links = append(links, attr.Val)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	return links
}

func isDescendantOfSkippableTag(n *html.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode {
			switch p.Data {
			case "script", "style", "link", "head", "noscript", "template", "nav", "footer", "aside", "button":
				return true
			}
		}
	}
	return false
}
