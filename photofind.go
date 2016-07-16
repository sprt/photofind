package photofind

import (
	"encoding/json"
	"html/template"
	"net/http"
	"os"

	"github.com/gorilla/csrf"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
)

const (
	maxImageSize = 2 << 20
	maxImages    = 4
)

var (
	secret    = must(os.Getenv("SECRET"))
	templates = template.Must(template.ParseFiles(
		"templates/index.html",
	))
)

type appHandler func(context.Context, http.ResponseWriter, *http.Request) error

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	if err := fn(ctx, w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func init() {
	r := http.NewServeMux()
	r.Handle("/", appHandler(indexHandler))
	r.Handle("/find", appHandler(findHandler))
	http.Handle("/", csrf.Protect([]byte(secret), csrf.Secure(!appengine.IsDevAppServer()))(r))
}

func indexHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	return renderTemplate(w, "index", map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(r),
	})
}

func findHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}

	err := r.ParseMultipartForm(maxImages * maxImageSize)
	if err != nil {
		return err
	}

	imgs, ok := r.MultipartForm.File["images"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}

	resps, err := annotateImages(ctx, imgs)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(resps)
}

func renderTemplate(w http.ResponseWriter, tpl string, data interface{}) error {
	return templates.ExecuteTemplate(w, tpl+".html", data)
}

func must(s string) string {
	if s != "" {
		return s
	}
	panic("s is the empty string")
}
