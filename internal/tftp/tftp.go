package tftp

import (
	"fmt"
	"github.com/pin/tftp"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

const MSG_TFTP_SS = "Starting embedded TFTP server on %s. Serving from %s..."

const ERR_PATH_V = "Failed to resolve docRoot: %v"
const ERR_TFTP_V = "TFTP Server failed: %v"
const ERR_TFTP_FILE_SV = "TFTP Error opening file %s: %v"

var docRoot string

func Server(srvIP net.IPNet, d string, logs chan string) error {
	var err error
	docRoot, err = filepath.Abs(d)
	if err != nil {
		return fmt.Errorf(ERR_PATH_V, err)
	}
	s := tftp.NewServer(readHandler, nil)
	s.SetTimeout(3 * time.Second)
	m := net.JoinHostPort(srvIP.IP.String(), "69")
	logs <- fmt.Sprintf(MSG_TFTP_SS, m, docRoot)
	err = s.ListenAndServe(m)
	if err != nil {
		return fmt.Errorf(ERR_TFTP_V, err)
	}
	return nil
}

func readHandler(filename string, rf io.ReaderFrom) error {
	cleanPath := filepath.Clean(filename)
	fullPath := filepath.Join(docRoot, cleanPath)
	file, err := os.Open(fullPath)
	if err != nil {
		return fmt.Errorf(ERR_TFTP_FILE_SV, filename, err)
	}
	defer file.Close()
	_, err = rf.ReadFrom(file)
	return err
}
