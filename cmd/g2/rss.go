package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"time"
)

type FeedItem struct {
	Title       string
	Link        string
	Description string
	PubDate     string
	Updated     string
}

type FeedData struct {
	Title         string
	Link          string
	Description   string
	LastBuildDate string
	Updated       string
	Items         []FeedItem
}

func generateFeeds(outPath, feedTitle, feedDescription, linkBase string, items []FeedItem) error {
	log.Printf("Generating feeds for %s at %s", feedTitle, outPath)
	now := time.Now()
	data := FeedData{
		Title:         feedTitle,
		Link:          linkBase,
		Description:   feedDescription,
		LastBuildDate: now.Format(time.RFC1123Z),
		Updated:       now.Format(time.RFC3339),
		Items:         items,
	}

	tmpl, err := template.ParseFS(siteTemplates, "sitegen_templates/*.xml")
	if err != nil {
		return fmt.Errorf("parsing feed templates for %s: %w", feedTitle, err)
	}

	var rssBuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&rssBuf, "rss.xml", data); err != nil {
		return fmt.Errorf("executing rss template for %s: %w", feedTitle, err)
	}
	if err := os.WriteFile(outPath+".rss", rssBuf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing rss file for %s at %s.rss: %w", feedTitle, outPath, err)
	}

	var atomBuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&atomBuf, "atom.xml", data); err != nil {
		return fmt.Errorf("executing atom template for %s: %w", feedTitle, err)
	}
	if err := os.WriteFile(outPath+".atom", atomBuf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing atom file for %s at %s.atom: %w", feedTitle, outPath, err)
	}

	return nil
}
