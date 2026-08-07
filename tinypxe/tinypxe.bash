#!/bin/bash
set -euo pipefail

sudo chmod u+s /usr/bin/mknod
ansible-playbook -ilocalhost, tinypxe.yml "$@"
sudo chmod u-s /usr/bin/mknod
