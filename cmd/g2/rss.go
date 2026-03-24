package main

import (
	"bytes"
	"html/template"
	"os"
	"time"
)

const rssTemplateStr = `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
<channel>
  <title>{{.Title}}</title>
  <link>{{.Link}}</link>
  <description>{{.Description}}</description>
  <lastBuildDate>{{.LastBuildDate}}</lastBuildDate>
  {{range .Items}}
  <item>
    <title>{{.Title}}</title>
    <link>{{.Link}}</link>
    <description>{{.Description}}</description>
    <pubDate>{{.PubDate}}</pubDate>
  </item>
  {{end}}
</channel>
</rss>
`

const atomTemplateStr = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>{{.Title}}</title>
  <link href="{{.Link}}"/>
  <updated>{{.Updated}}</updated>
  <id>{{.Link}}</id>
  {{range .Items}}
  <entry>
    <title>{{.Title}}</title>
    <link href="{{.Link}}"/>
    <id>{{.Link}}</id>
    <updated>{{.Updated}}</updated>
    <summary>{{.Description}}</summary>
  </entry>
  {{end}}
</feed>
`

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
	now := time.Now()
	data := FeedData{
		Title:         feedTitle,
		Link:          linkBase,
		Description:   feedDescription,
		LastBuildDate: now.Format(time.RFC1123Z),
		Updated:       now.Format(time.RFC3339),
		Items:         items,
	}

	rssTmpl, err := template.New("rss").Parse(rssTemplateStr)
	if err != nil {
		return err
	}
	var rssBuf bytes.Buffer
	if err := rssTmpl.Execute(&rssBuf, data); err != nil {
		return err
	}
	if err := os.WriteFile(outPath+".rss", rssBuf.Bytes(), 0644); err != nil {
		return err
	}

	atomTmpl, err := template.New("atom").Parse(atomTemplateStr)
	if err != nil {
		return err
	}
	var atomBuf bytes.Buffer
	if err := atomTmpl.Execute(&atomBuf, data); err != nil {
		return err
	}
	if err := os.WriteFile(outPath+".atom", atomBuf.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}
