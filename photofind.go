package photofind

import (
	"html/template"
	"net/http"
	"os"

	"github.com/gorilla/csrf"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

var secret = must(os.Getenv("SECRET"))

var templates = template.Must(template.ParseFiles(
	"templates/index.html",
))

func init() {
	r := http.NewServeMux()
	r.HandleFunc("/", indexHandler)
	http.Handle("/", csrf.Protect([]byte(secret), csrf.Secure(!appengine.IsDevAppServer()))(r))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		err := r.ParseMultipartForm(1 << 20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		imgs, ok := r.MultipartForm.File["images"]
		if !ok {
			http.Error(w, "No images", http.StatusBadRequest)
			return
		}

		c := appengine.NewContext(r)
		for _, img := range imgs {
			log.Debugf(c, img.Filename)
		}
	}

	renderTemplate(w, "index", map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(r),
	})
}

func renderTemplate(w http.ResponseWriter, tpl string, data interface{}) {
	err := templates.ExecuteTemplate(w, tpl+".html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func must(s string) string {
	if s != "" {
		return s
	}
	panic("s is the empty string")
}
