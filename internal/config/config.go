// Package config parses the command line into one Config shared by
// the servers.
package config

import (
	"flag"
	"fmt"
	"net"
	"strings"
)

// Config is the runtime configuration.
type Config struct {
	Interface    string
	ServerIP     net.IP
	Mask         net.IPMask
	WWW          string
	TFTPRoot     string
	DB           string
	Seed         string
	DNS          []net.IP
	VoidUpstream string
	KernelArgs   string
	HTTPAddr     string
	TFTPAddr     string
	LeaseSecs    uint32
}

// Parse reads flags from args (without the program name).
func Parse(args []string) (*Config, error) {
	c := &Config{}
	fs := flag.NewFlagSet("iolite", flag.ContinueOnError)
	cidr := fs.String("ip", "172.168.16.1/16",
		"server address and subnet in CIDR form")
	fs.StringVar(&c.Interface, "iface", "virbr0",
		"interface to answer DHCP on")
	fs.StringVar(&c.WWW, "www", "www",
		"HTTP document root; kernel and initrd live in boot/")
	fs.StringVar(&c.TFTPRoot, "tftp", "tftpboot",
		"TFTP root holding the iPXE binaries")
	fs.StringVar(&c.DB, "db", "profiles.db", "SQLite database")
	fs.StringVar(&c.Seed, "seed", "seed/profiles.sql",
		"SQL loaded into an empty database (empty to skip)")
	dns := fs.String("dns", "",
		"comma-separated DNS servers to offer (none by default)")
	fs.StringVar(&c.VoidUpstream, "void-upstream",
		"https://repo-default.voidlinux.org",
		"upstream proxied at /void/")
	fs.StringVar(&c.KernelArgs, "kernel-args",
		"console=ttyS0,115200 console=tty0",
		"kernel command line in boot.ipxe")
	fs.StringVar(&c.HTTPAddr, "http", "",
		"HTTP listen address (default <ip>:80)")
	fs.StringVar(&c.TFTPAddr, "tftp-addr", "",
		"TFTP listen address (default <ip>:69)")
	lease := fs.Uint("lease", 3600, "DHCP lease seconds")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	c.LeaseSecs = uint32(*lease)
	if err := c.setNetwork(*cidr, *dns); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) setNetwork(cidr, dns string) error {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("-ip: %w", err)
	}
	c.ServerIP = ip.To4()
	if c.ServerIP == nil {
		return fmt.Errorf("-ip: %s is not IPv4", cidr)
	}
	c.Mask = ipnet.Mask
	host := c.ServerIP.String()
	if c.HTTPAddr == "" {
		c.HTTPAddr = net.JoinHostPort(host, "80")
	}
	if c.TFTPAddr == "" {
		c.TFTPAddr = net.JoinHostPort(host, "69")
	}
	for _, s := range strings.Split(dns, ",") {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		d := net.ParseIP(s)
		if d == nil {
			return fmt.Errorf("-dns: bad address %q", s)
		}
		c.DNS = append(c.DNS, d.To4())
	}
	return nil
}

// URLBase is the http://host[:port] clients should use.
func (c *Config) URLBase() string {
	host, port, err := net.SplitHostPort(c.HTTPAddr)
	if err != nil || host == "" || host == "0.0.0.0" {
		host = c.ServerIP.String()
	}
	if port == "80" || port == "" {
		return "http://" + host
	}
	return "http://" + net.JoinHostPort(host, port)
}

// BootURL is where iPXE clients fetch their script.
func (c *Config) BootURL() string {
	return c.URLBase() + "/boot/boot.ipxe"
}

// RepoDefault is the proxied Void repository for profiles that do
// not name one.
func (c *Config) RepoDefault() string {
	return c.URLBase() + "/void/current"
}
