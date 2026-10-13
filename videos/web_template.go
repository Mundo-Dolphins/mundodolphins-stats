package videos

import (
	_ "embed"
	"html/template"
)

//go:embed web_index.html
var webIndexHTML string

var webIndexTemplate = template.Must(template.New("videos-web").Parse(webIndexHTML))
