package httpd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"iolite/internal/store"
)

const mac = "52:54:00:76:de:e5"

type fixture struct {
	srv      *httptest.Server
	upstream *httptest.Server
	seen     []string
}

func setup(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SeedIfEmpty("../../seed/profiles.sql"); err != nil {
		t.Fatal(err)
	}
	boot := filepath.Join(dir, "boot")
	os.MkdirAll(boot, 0o755)
	os.WriteFile(filepath.Join(boot, "vmlinuz"), []byte("K"), 0o644)
	f := &fixture{}
	f.upstream = httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			f.seen = append(f.seen, r.Host+" "+r.URL.Path)
			io.WriteString(w, "repodata")
		}))
	t.Cleanup(f.upstream.Close)
	logs := make(chan string, 100)
	h, err := New(st, Options{
		WWW:          dir,
		URLBase:      "http://h",
		KernelArgs:   "console=tty0",
		RepoDefault:  "http://h/void/current",
		VoidUpstream: f.upstream.URL,
	}, logs)
	if err != nil {
		t.Fatal(err)
	}
	f.srv = httptest.NewServer(h)
	t.Cleanup(f.srv.Close)
	return f
}

func get(t *testing.T, u string) (int, string) {
	t.Helper()
	res, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestBootAndStatic(t *testing.T) {
	f := setup(t)
	code, body := get(t, f.srv.URL+"/boot/boot.ipxe")
	if code != 200 || !strings.Contains(body,
		"kernel http://h/boot/vmlinuz console=tty0\n") {
		t.Fatalf("%d %q", code, body)
	}
	code, body = get(t, f.srv.URL+"/boot/vmlinuz")
	if code != 200 || body != "K" {
		t.Fatalf("%d %q", code, body)
	}
	if code, _ = get(t, f.srv.URL+"/boot/missing"); code != 404 {
		t.Fatalf("missing: %d", code)
	}
}

func TestProfileLifecycle(t *testing.T) {
	f := setup(t)
	code, body := get(t, f.srv.URL+"/profiles/"+mac)
	if code != 200 || !strings.Contains(body, "DISKS='vda'\n") ||
		!strings.Contains(body, "REPO='http://h/void/current'\n") {
		t.Fatalf("%d %q", code, body)
	}
	if code, _ = get(t, f.srv.URL+"/profiles/00:11:22:33:44:55"); code != 404 {
		t.Fatalf("unknown mac: %d", code)
	}
	res, err := http.PostForm(f.srv.URL+"/profiles/"+mac+"/status",
		url.Values{"state": {"done"}})
	if err != nil || res.StatusCode != 204 {
		t.Fatalf("done: %v %v", res, err)
	}
	if code, _ = get(t, f.srv.URL+"/profiles/"+mac); code != 404 {
		t.Fatalf("after done: %d", code)
	}
	res, err = http.PostForm(f.srv.URL+"/profiles/"+mac+"/status",
		url.Values{"state": {"bogus"}})
	if err != nil || res.StatusCode != 400 {
		t.Fatalf("bogus state: %v %v", res, err)
	}
}

func TestVoidProxy(t *testing.T) {
	f := setup(t)
	code, body := get(t, f.srv.URL+"/void/current/x86_64-repodata")
	if code != 200 || body != "repodata" {
		t.Fatalf("%d %q", code, body)
	}
	up, _ := url.Parse(f.upstream.URL)
	want := up.Host + " /current/x86_64-repodata"
	if len(f.seen) != 1 || f.seen[0] != want {
		t.Fatalf("upstream saw %v, want %q", f.seen, want)
	}
}
