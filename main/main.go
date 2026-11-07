// Command iolite runs the DHCP, TFTP and HTTP servers that PXE-boot
// known machines into the installer image.
package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"iolite/internal/config"
	"iolite/internal/dhcp"
	"iolite/internal/httpd"
	"iolite/internal/store"
	"iolite/internal/tftp"
)

func main() {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		os.Exit(2)
	}
	logs := make(chan string, 64)
	st, err := openStore(cfg, logs)
	if err != nil {
		log.Fatal(err)
	}
	h, err := httpd.New(st, httpd.Options{
		WWW:          cfg.WWW,
		URLBase:      cfg.URLBase(),
		KernelArgs:   cfg.KernelArgs,
		RepoDefault:  cfg.RepoDefault(),
		VoidUpstream: cfg.VoidUpstream,
	}, logs)
	if err != nil {
		log.Fatal(err)
	}
	d := dhcp.New(dhcp.Config{
		Interface: cfg.Interface,
		ServerIP:  cfg.ServerIP,
		Mask:      cfg.Mask,
		DNS:       cfg.DNS,
		LeaseSecs: cfg.LeaseSecs,
		HTTPBoot:  cfg.BootURL(),
	}, lookup(st), logs)
	t := tftp.New(cfg.TFTPAddr, cfg.TFTPRoot, logs)
	serve(cfg, d, t, h, logs)
}

// serve runs the three servers and prints their logs until one of
// them fails.
func serve(cfg *config.Config, d *dhcp.Server, t *tftp.Server,
	h http.Handler, logs chan string) {
	fatal := make(chan error, 3)
	go func() { fatal <- d.Run() }()
	go func() { fatal <- t.Run() }()
	go func() {
		logs <- fmt.Sprintf("Serving HTTP on %s", cfg.HTTPAddr)
		fatal <- http.ListenAndServe(cfg.HTTPAddr, h)
	}()
	for {
		select {
		case msg := <-logs:
			log.Println(msg)
		case err := <-fatal:
			log.Fatal(err)
		}
	}
}

func openStore(cfg *config.Config, logs chan string) (*store.Store,
	error) {
	st, err := store.Open(cfg.DB)
	if err != nil {
		return nil, err
	}
	if err := st.Migrate(); err != nil {
		return nil, err
	}
	if cfg.Seed != "" {
		seeded, err := st.SeedIfEmpty(cfg.Seed)
		if err != nil {
			return nil, err
		}
		if seeded {
			logs <- "Seeded empty database from " + cfg.Seed
		}
	}
	return st, nil
}

// lookup answers DHCP only for enabled profiles.
func lookup(st *store.Store) dhcp.Lookup {
	return func(mac net.HardwareAddr) (net.IP, bool) {
		p, err := st.ProfileByMAC(mac.String())
		if err != nil || !p.IsBuildEnabled {
			return nil, false
		}
		ip := net.ParseIP(p.IPAddress)
		return ip, ip != nil
	}
}
