package http

import (
	"fmt"
	"net"
	nhttp "net/http"
	"path/filepath"
)

const MSG_HTTP_SS = "Starting HTTP server on %s. Serving from %s..."

const ERR_HTTP_V = "HTTP Server failed: %v"
const ERR_PATH_V = "Failed to resolve docRoot: %v"

// TODO: modify this to provide an API
func Server(srvIP net.IPNet, docRoot string, logs chan string) error {
	var err error
	docRoot, err = filepath.Abs(docRoot)
	if err != nil {
		return fmt.Errorf(ERR_PATH_V, err)
	}
	fileServer := nhttp.FileServer(nhttp.Dir(docRoot))
	m := net.JoinHostPort(srvIP.IP.String(), "80")
	logs <- fmt.Sprintf(MSG_HTTP_SS, m, docRoot)
	err = nhttp.ListenAndServe(m, fileServer)
	if err != nil {
		return fmt.Errorf(ERR_HTTP_V, err)
	}
	return nil
}
