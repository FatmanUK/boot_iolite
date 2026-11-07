// Package tftp serves the iPXE binaries read-only to firmware PXE
// ROMs, which cannot speak anything else.
package tftp

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pin/tftp"
)

// Server serves files below Root.
type Server struct {
	Addr string
	Root string
	logs chan<- string
}

// New prepares a server; Run starts it.
func New(addr, root string, logs chan<- string) *Server {
	return &Server{Addr: addr, Root: root, logs: logs}
}

// Run serves forever; it returns only on a setup error.
func (s *Server) Run() error {
	conn, err := s.Listen()
	if err != nil {
		return err
	}
	return s.Serve(conn)
}

// Listen binds Addr and resolves Root.
func (s *Server) Listen() (*net.UDPConn, error) {
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return nil, fmt.Errorf("tftp root: %w", err)
	}
	s.Root = root
	addr, err := net.ResolveUDPAddr("udp4", s.Addr)
	if err != nil {
		return nil, fmt.Errorf("tftp addr %q: %w", s.Addr, err)
	}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return nil, fmt.Errorf("tftp listen: %w", err)
	}
	return conn, nil
}

// Serve answers read requests on conn.
func (s *Server) Serve(conn *net.UDPConn) error {
	srv := tftp.NewServer(s.read, nil)
	srv.SetTimeout(3 * time.Second)
	s.logs <- fmt.Sprintf("Serving TFTP on %s from %s",
		conn.LocalAddr(), s.Root)
	srv.Serve(conn)
	return fmt.Errorf("tftp: server on %s stopped", conn.LocalAddr())
}

func (s *Server) read(name string, rf io.ReaderFrom) error {
	clean := filepath.Clean("/" + name)
	full := filepath.Join(s.Root, clean)
	if !strings.HasPrefix(full, s.Root+string(os.PathSeparator)) {
		return fmt.Errorf("tftp: refusing %q", name)
	}
	f, err := os.Open(full)
	if err != nil {
		s.logs <- fmt.Sprintf("TFTP %s: %v", name, err)
		return err
	}
	defer f.Close()
	n, err := rf.ReadFrom(f)
	if err == nil {
		s.logs <- fmt.Sprintf("TFTP sent %s (%d bytes)", clean, n)
	}
	return err
}
