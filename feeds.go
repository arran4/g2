package g2

type FeedItem struct {
	ID          string
	Title       string
	Link        string
	Description string
	PubDate     string
	Updated     string
}

type Feed struct {
	ID            string
	Title         string
	Link          string
	Description   string
	LastBuildDate string
	Updated       string
	Items         []FeedItem
}
