package main

import (
	"bytes"
	"github.com/arran4/g2"
	"html/template"
	"os"
	"time"
)

type FeedItem struct {
	Title       string
	Link        string
	Description string
	PubDate     string
	Updated     string
	Time        time.Time
}

type FeedData struct {
	Title         string
	Link          string
	Description   string
	LastBuildDate string
	Updated       string
	Items         []FeedItem
}

func generateFeeds(outPath, feedTitle, feedDescription, linkBase string, items []g2.FeedItem) error {
	now := time.Now()
	data := g2.Feed{
		Title:         feedTitle,
		Link:          linkBase,
		Description:   feedDescription,
		LastBuildDate: now.Format(time.RFC1123Z),
		Updated:       now.Format(time.RFC3339),
		Items:         items,
	}

	tmpl, err := template.ParseFS(siteTemplates, "sitegen_templates/*.xml")
	if err != nil {
		return err
	}

	var rssBuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&rssBuf, "rss.xml", data); err != nil {
		return err
	}
	if err := os.WriteFile(outPath+".rss", rssBuf.Bytes(), 0644); err != nil {
		return err
	}

	var atomBuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&atomBuf, "atom.xml", data); err != nil {
		return err
	}
	if err := os.WriteFile(outPath+".atom", atomBuf.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}
