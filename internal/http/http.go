package http

import (
	"fmt"
	"log"
	nhttp "net/http"
)

const MSG_HTTP_SS = "Starting HTTP server on %s:%s..."

const ERR_HTTP_V = "HTTP Server failed to start: %v"

func Server(srvIP string, docRoot string) {
	fileServer := nhttp.FileServer(nhttp.Dir(docRoot))
	log.Printf(MSG_HTTP_SS, srvIP)
	m := fmt.Sprintf("%s:%s", srvIP, "80")
	err := nhttp.ListenAndServe(m, fileServer)
	if err != nil {
		log.Fatalf(ERR_HTTP_V, err)
	}
}
