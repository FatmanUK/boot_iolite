package tftp

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/pin/tftp"
)

func TestServe(t *testing.T) {
	root := t.TempDir()
	want := bytes.Repeat([]byte("ipxe"), 1000)
	err := os.WriteFile(filepath.Join(root, "undionly.kpxe"), want,
		0o644)
	if err != nil {
		t.Fatal(err)
	}
	logs := make(chan string, 10)
	s := New("127.0.0.1:0", root, logs)
	conn, err := s.Listen()
	if err != nil {
		t.Fatal(err)
	}
	go s.Serve(conn)
	defer conn.Close()

	c, err := tftp.NewClient(conn.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	wt, err := c.Receive("undionly.kpxe", "octet")
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if _, err := wt.WriteTo(&got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("got %d bytes, want %d", got.Len(), len(want))
	}
	if _, err := c.Receive("../tftp_test.go", "octet"); err == nil {
		t.Fatal("path traversal was not refused")
	}
	if _, err := c.Receive("missing", "octet"); err == nil {
		t.Fatal("missing file did not error")
	}
}
