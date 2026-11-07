// Package httpd serves everything after the firmware stage: the iPXE
// script, kernel and initrd, per-machine profiles, the install-done
// callback, and a reverse proxy to the Void package repository.
package httpd

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"

	"iolite/internal/render"
	"iolite/internal/store"
)

// Options configures the handler.
type Options struct {
	WWW          string // document root; boot/ inside it
	URLBase      string // http://host[:port] as clients see it
	KernelArgs   string
	RepoDefault  string
	VoidUpstream string
}

// Handler is the HTTP mux with its dependencies.
type Handler struct {
	st   *store.Store
	opt  Options
	logs chan<- string
	mux  *http.ServeMux
}

// New builds the handler; it fails only on a bad upstream URL.
func New(st *store.Store, opt Options, logs chan<- string,
) (*Handler, error) {
	up, err := url.Parse(opt.VoidUpstream)
	if err != nil || up.Host == "" {
		return nil, fmt.Errorf("void upstream %q: %v",
			opt.VoidUpstream, err)
	}
	h := &Handler{st: st, opt: opt, logs: logs,
		mux: http.NewServeMux()}
	boot := http.Dir(filepath.Join(opt.WWW, "boot"))
	h.mux.HandleFunc("GET /boot/boot.ipxe", h.bootScript)
	h.mux.Handle("GET /boot/",
		http.StripPrefix("/boot/", http.FileServer(boot)))
	h.mux.HandleFunc("GET /profiles/{mac}", h.profile)
	h.mux.HandleFunc("POST /profiles/{mac}/status", h.status)
	h.mux.Handle("/void/", http.StripPrefix("/void", proxy(up)))
	h.mux.HandleFunc("GET /{$}", h.index)
	return h, nil
}

func proxy(up *url.URL) http.Handler {
	return &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(up)
		},
	}
}

// ServeHTTP logs each request and dispatches it.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rec := &recorder{ResponseWriter: w, status: http.StatusOK}
	h.mux.ServeHTTP(rec, r)
	h.logs <- fmt.Sprintf("HTTP %s %s %s %d", r.RemoteAddr,
		r.Method, r.URL.Path, rec.status)
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintln(w, "iolite: /boot/ /profiles/<mac> /void/ /admin/")
}

func (h *Handler) bootScript(w http.ResponseWriter,
	r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	script := render.BootScript(h.opt.URLBase, h.opt.KernelArgs)
	fmt.Fprint(w, script)
}

func fail(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (h *Handler) profile(w http.ResponseWriter, r *http.Request) {
	p, err := h.st.ProfileByMAC(r.PathValue("mac"))
	if errors.Is(err, store.ErrNotFound) || err == nil &&
		!p.IsBuildEnabled {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	d := render.Defaults{Repo: h.opt.RepoDefault}
	fmt.Fprint(w, render.ProfileEnv(p, d))
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	mac := r.PathValue("mac")
	if r.FormValue("state") != "done" {
		http.Error(w, "state must be done", http.StatusBadRequest)
		return
	}
	err := h.st.MarkDone(mac)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	h.logs <- fmt.Sprintf("Install done for %s; profile disabled", mac)
	w.WriteHeader(http.StatusNoContent)
}
