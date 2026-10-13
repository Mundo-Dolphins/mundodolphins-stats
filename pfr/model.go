package pfr

// LinkRef represents a link reference with its anchor text.
type LinkRef struct {
	Anchor string `json:"anchor"`
	Link   string `json:"link"`
}

// LinkValue represents a simple link in a table cell.
type LinkValue struct {
	Content string `json:"content"`
	Link    string `json:"link"`
}
