package photofind

import (
	"html/template"
	"net/http"
)

var templates = template.Must(template.ParseFiles(
	"templates/index.html",
))

func init() {
	http.HandleFunc("/", indexHandler)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "index", nil)
}

func renderTemplate(w http.ResponseWriter, tpl string, data interface{}) {
	err := templates.ExecuteTemplate(w, tpl+".html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
