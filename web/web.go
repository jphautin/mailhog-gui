package web

import (
	"bytes"
	"embed"
	"github.com/ian-kent/go-log/log"
	"golang.org/x/crypto/bcrypt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/pat"
	"github.com/jphautin/mailhog-gui/config"
)

var APIHost string
var WebPath string

type Web struct {
	config *config.Config
	asset  func(string) ([]byte, error)
}

// AuthFile sets Authorised to a function which validates against file
func AuthFile(file string) {
	users = make(map[string]string)

	b, err := os.ReadFile(file)
	if err != nil {
		log.Fatal("[HTTP] Error reading auth-file: %s", err)
		// FIXME - go-log
		os.Exit(1)
	}

	buf := bytes.NewBuffer(b)

	for {
		l, err := buf.ReadString('\n')
		l = strings.TrimSpace(l)
		if len(l) > 0 {
			p := strings.SplitN(l, ":", 2)
			if len(p) < 2 {
				log.Fatal("[HTTP] Error reading auth-file, invalid line: %s", l)
				// FIXME - go-log
				os.Exit(1)
			}
			users[p[0]] = p[1]
		}
		switch {
		case err == io.EOF:
			break
		case err != nil:
			log.Fatal("[HTTP] Error reading auth-file: %s", err)
			// FIXME - go-log
			os.Exit(1)
		}
		if err == io.EOF {
			break
		} else if err != nil {
		}
	}

	log.Info("[HTTP] Loaded %d users from %s", len(users), file)

	Authorised = func(u, pw string) bool {
		hpw, ok := users[u]
		if !ok {
			return false
		}

		err := bcrypt.CompareHashAndPassword([]byte(hpw), []byte(pw))
		if err != nil {
			return false
		}

		return true
	}
}
func Listen(httpBindAddr string, registerCallback func(http.Handler)) {
	log.Info("[HTTP] Binding to address: %s", httpBindAddr)

	vpat := pat.New()
	registerCallback(vpat)

	//compress := handlers.CompressHandler(pat)
	auth := BasicAuthHandler(vpat) //compress)

	err := http.ListenAndServe(httpBindAddr, auth)
	if err != nil {
		log.Fatal("[HTTP] Error binding to address %s: %s", httpBindAddr, err)
	}
}

var Authorised func(string, string) bool
var users map[string]string

// BasicAuthHandler is middleware to check HTTP Basic Authentication
// if an authorisation function is defined.
func BasicAuthHandler(h http.Handler) http.Handler {
	f := func(w http.ResponseWriter, req *http.Request) {
		if Authorised == nil {
			h.ServeHTTP(w, req)
			return
		}

		u, pw, ok := req.BasicAuth()
		if !ok || !Authorised(u, pw) {
			w.Header().Set("WWW-Authenticate", "Basic")
			w.WriteHeader(401)
			return
		}
		h.ServeHTTP(w, req)
	}

	return http.HandlerFunc(f)
}

// content holds our static web server content.
//
//go:embed assets/*
var content embed.FS

func CreateWeb(cfg *config.Config, r http.Handler) *Web {
	web := &Web{
		config: cfg,
		asset:  content.ReadFile,
	}

	pat := r.(*pat.Router)

	WebPath = cfg.WebPath

	log.Info("Serving under http://%s%s/", cfg.UIBindAddr, WebPath)

	pat.Path(WebPath + "/images/{file:.*}").Methods("GET").HandlerFunc(web.Static("assets/images/{{file}}"))
	pat.Path(WebPath + "/css/{file:.*}").Methods("GET").HandlerFunc(web.Static("assets/css/{{file}}"))
	pat.Path(WebPath + "/js/{file:.*}").Methods("GET").HandlerFunc(web.Static("assets/js/{{file}}"))
	pat.Path(WebPath + "/fonts/{file:.*}").Methods("GET").HandlerFunc(web.Static("assets/fonts/{{file}}"))
	pat.StrictSlash(true).Path(WebPath + "/").Methods("GET").HandlerFunc(web.Index())

	return web
}

func (web Web) Static(pattern string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		fp := strings.TrimSuffix(pattern, "{{file}}") + req.URL.Query().Get(":file")
		if b, err := web.asset(fp); err == nil {
			ext := filepath.Ext(fp)

			w.Header().Set("Content-Type", mime.TypeByExtension(ext))
			w.WriteHeader(200)
			_, err := w.Write(b)
			if err != nil {
				log.Warn("error sending file %s", fp)
				return
			}
			return
		}
		log.Info("[UI] File not found: %s", fp)
		w.WriteHeader(404)
	}
}

func (web Web) Index() func(http.ResponseWriter, *http.Request) {
	tmpl := template.New("index.html")
	tmpl.Delims("[:", ":]")

	asset, err := web.asset("assets/templates/index.html")
	if err != nil {
		log.Fatal("[UI] Error loading index.html: %s", err)
	}

	tmpl, err = tmpl.Parse(string(asset))
	if err != nil {
		log.Fatal("[UI] Error parsing index.html: %s", err)
	}

	layout := template.New("layout.html")
	layout.Delims("[:", ":]")

	asset, err = web.asset("assets/templates/layout.html")
	if err != nil {
		log.Fatal("[UI] Error loading layout.html: %s", err)
	}

	layout, err = layout.Parse(string(asset))
	if err != nil {
		log.Fatal("[UI] Error parsing layout.html: %s", err)
	}

	return func(w http.ResponseWriter, req *http.Request) {
		data := map[string]interface{}{
			"config":  web.config,
			"Page":    "Browse",
			"APIHost": APIHost,
		}

		b := new(bytes.Buffer)
		err := tmpl.Execute(b, data)

		if err != nil {
			log.Printf("[UI] Error executing template: %s", err)
			w.WriteHeader(500)
			return
		}

		data["Content"] = template.HTML(b.String())

		b = new(bytes.Buffer)
		err = layout.Execute(b, data)

		if err != nil {
			log.Printf("[UI] Error executing template: %s", err)
			w.WriteHeader(500)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		_, err = w.Write(b.Bytes())
		if err != nil {
			return
		}
	}
}
