package config

import "testing"

func TestDefaults(t *testing.T) {
	c, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.ServerIP.String() != "172.168.16.1" {
		t.Errorf("ip %s", c.ServerIP)
	}
	if ones, _ := c.Mask.Size(); ones != 16 {
		t.Errorf("mask %v", c.Mask)
	}
	if c.HTTPAddr != "172.168.16.1:80" || c.TFTPAddr != "172.168.16.1:69" {
		t.Errorf("addrs %s %s", c.HTTPAddr, c.TFTPAddr)
	}
	if c.BootURL() != "http://172.168.16.1/boot/boot.ipxe" {
		t.Errorf("boot url %s", c.BootURL())
	}
	if c.RepoDefault() != "http://172.168.16.1/void/current" {
		t.Errorf("repo %s", c.RepoDefault())
	}
}

func TestOverrides(t *testing.T) {
	c, err := Parse([]string{"-ip", "10.0.0.1/24", "-http", ":8080",
		"-dns", "10.0.0.1, 10.0.0.2"})
	if err != nil {
		t.Fatal(err)
	}
	if c.URLBase() != "http://10.0.0.1:8080" {
		t.Errorf("url base %s", c.URLBase())
	}
	if len(c.DNS) != 2 || c.DNS[1].String() != "10.0.0.2" {
		t.Errorf("dns %v", c.DNS)
	}
}

func TestBadInput(t *testing.T) {
	for _, args := range [][]string{
		{"-ip", "nope"},
		{"-ip", "2001:db8::1/64"},
		{"-dns", "x"},
	} {
		if _, err := Parse(args); err == nil {
			t.Errorf("%v: expected error", args)
		}
	}
}
