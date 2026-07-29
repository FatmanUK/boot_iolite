#!/bin/bash
set -euo pipefail

ansible-playbook -ilocalhost, tinypxe.yml $@
