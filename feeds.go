package g2

type FeedItem struct {
	Title       string
	Link        string
	Description string
	PubDate     string
	Updated     string
}

type Feed struct {
	Title         string
	Link          string
	Description   string
	LastBuildDate string
	Updated       string
	Items         []FeedItem
}
