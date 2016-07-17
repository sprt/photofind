package photofind

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/csrf"
	"github.com/gorilla/securecookie"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/user"
)

const (
	accessCodeLife = 24 * time.Hour
	cookieLife     = 365 * 24 * time.Hour
	maxUploadSize  = 8 << 20
)

var (
	errAlreadyUsed = errors.New("access code already used")
	secret         = must(os.Getenv("SECRET"))
	templates      = template.Must(template.ParseFiles("templates/index.html"))
	s              = securecookie.New([]byte(secret), nil)
)

type AccessCode struct {
	CreatedAt      time.Time
	CreatedByEmail string
	CreatedByAdmin bool
	Used           bool
}

type appHandler func(context.Context, http.ResponseWriter, *http.Request) error

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	if err := fn(ctx, w, r); err != nil {
		log.Errorf(ctx, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func init() {
	r := http.NewServeMux()
	r.Handle("/", appHandler(indexHandler))
	r.Handle("/find", appHandler(findHandler))
	r.Handle("/share", appHandler(shareHandler))
	http.Handle("/", csrf.Protect([]byte(secret), csrf.Secure(!appengine.IsDevAppServer()))(r))
}

func indexHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	authorized := checkAccess(ctx, w, r)
	encodedKey := r.URL.Query().Get("code")
	cuser := user.Current(ctx)

	if !authorized && encodedKey != "" && (cuser == nil || !cuser.Admin) {
		key, err := datastore.DecodeKey(encodedKey)
		if err != nil {
			return err
		}

		var encodedCookie string
		err = datastore.RunInTransaction(ctx, func(ctx context.Context) error {
			code := new(AccessCode)
			err := datastore.Get(ctx, key, code)
			if err != nil {
				return err
			}

			if code.Used || time.Now().After(code.CreatedAt.Add(accessCodeLife)) {
				return errAlreadyUsed
			}

			code.Used = true
			_, err = datastore.Put(ctx, key, code)
			if err != nil {
				return err
			}

			encodedCookie, err = s.Encode("access_code", encodedKey)
			if err != nil {
				return err
			}

			return nil
		}, nil)
		if err != nil && err != datastore.ErrConcurrentTransaction {
			return err
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "access_code",
			Value:    encodedCookie,
			Expires:  time.Now().Add(cookieLife),
			MaxAge:   int(cookieLife),
			Secure:   !appengine.IsDevAppServer(),
			HttpOnly: true,
		})
		authorized = true
	}

	if !authorized {
		w.WriteHeader(http.StatusForbidden)
		return nil
	}

	return renderTemplate(w, "index", map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(r),
	})
}

func findHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if ok := checkAccess(ctx, w, r); !ok {
		w.WriteHeader(http.StatusForbidden)
		return nil
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusBadRequest)
		return nil
	}

	err := r.ParseMultipartForm(maxUploadSize)
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

func shareHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	cuser := user.Current(ctx)
	code := &AccessCode{
		CreatedAt:      time.Now(),
		CreatedByEmail: cuser.Email,
		CreatedByAdmin: cuser.Admin,
	}
	incomplete := datastore.NewIncompleteKey(ctx, "AccessCode", nil)

	key, err := datastore.Put(ctx, incomplete, code)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<a href="/?code=%s">Access link</a>`, key.Encode())
	return nil
}

func checkAccess(ctx context.Context, w http.ResponseWriter, r *http.Request) bool {
	cuser := user.Current(ctx)
	if cuser != nil && cuser.Admin {
		return true
	}

	cookie, err := r.Cookie("access_code")
	if err != nil {
		return false
	}

	var encodedKey string
	err = s.Decode("access_code", cookie.Value, &encodedKey)
	if err != nil {
		return false
	}

	key, err := datastore.DecodeKey(encodedKey)
	if err != nil {
		return false
	}

	err = datastore.Get(ctx, key, nil)
	return err == nil
}

func renderTemplate(w http.ResponseWriter, tpl string, data interface{}) error {
	return templates.ExecuteTemplate(w, tpl+".html", data)
}

func must(s string) string {
	if s == "" {
		panic("s is the empty string")
	}
	return s
}
