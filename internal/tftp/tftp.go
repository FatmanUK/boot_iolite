package tftp

import (
	"time"
	"log"
	"io"
	"os"
	"path/filepath"
	"github.com/pin/tftp"
)

const MSG_TFTP = "Starting embedded TFTP server on :69..."

const ERR_TFTP_V = "TFTP Server failed: %v"
const ERR_TFTP_FILE_SV = "TFTP Error opening file %s: %v"

var docRoot string

func Server(d string) {
	docRoot = d
	s := tftp.NewServer(readHandler, nil)
	s.SetTimeout(3 * time.Second)
	log.Println(MSG_TFTP)
	err := s.ListenAndServe(":69")
	if err != nil {
		log.Fatalf(ERR_TFTP_V, err)
	}
}

func readHandler(filename string, rf io.ReaderFrom) error {
	cleanPath := filepath.Clean(filename)
	fullPath := filepath.Join(docRoot, cleanPath)
	file, err := os.Open(fullPath)
	if err != nil {
		log.Printf(ERR_TFTP_FILE_SV, filename, err)
		return err
	}
	defer file.Close()
	_, err = rf.ReadFrom(file)
	return err
}
