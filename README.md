# iolite

Self-contained DHCP, TFTP and HTTP server, the purpose of which is to PXE-boot an ultra-light Linux distro sized at around 7.5MB. It boots a QEMU VM to the sh prompt in less than a second. (You need to put a serial console in the VM and switch to serial console view after boot is complete.)

A 7.5MB distro? Sure, if a minimal kernel plus statically-linked Busybox and Runit and not much else can be called a distro. Hmm, now what can I do with this?

## Features to add

  * efi bootloader
  * shun unknown MAC addresses

## Bugs

  * serial console only --- I'd like to use that AND tty0 if possible
