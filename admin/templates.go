package admin

import (
	"embed"
	"html/template"

	"github.com/Masterminds/sprig/v3"
)

//go:embed views
var fs embed.FS
var (
	templates *template.Template
)

func init() {
	sprig.FuncMap()
	templates = template.Must(template.New("").Funcs(sprig.FuncMap()).ParseFS(fs, "views/*.go.html"))
}

// Tmpls returns templates
func Tmpls() *template.Template {
	return templates
}
