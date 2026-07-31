from __future__ import (absolute_import, division, print_function)
__metaclass__ = type

import os
import sys
import gzip
import shutil
import tempfile
import subprocess

DOCUMENTATION = r'''
---
module: initrd_build
short_description: Write initrd.img
version_added: "0.0.1"
description: Write an initrd.img file from a stage directory. Needs
    cpio installed at /usr/bin/cpio.
options:
    chdir:
        description: The root directory with the initrd working files.
        required: true
        type: str
    dest:
        description: The initrd file to write.
        required: true
        type: str
author:
    - Adam J. Richardson (@FatmanUK)
'''

EXAMPLES = r'''
# Build initrd from a directory
- name: Test with a message
  initrd_build:
    chdir: "/tmp/build_initrd"
    dest: "/tmp/build_iso/boot/initrd.img"
'''

RETURN = r'''
files:
    description: The number of files compressed.
    type: int
    returned: always
    sample: 31
size:
    description: Size of the compressed initrd image /bytes.
    type: int
    returned: always
    sample: 12345
'''

from ansible.module_utils.basic import AnsibleModule

def run_module():
    module_args = dict(
        chdir=dict(type='str', required=True),
        dest=dict(type='str', required=True))
    result = dict(changed=False, files=0, size=0)
    module = AnsibleModule(
        argument_spec=module_args,
        supports_check_mode=True)
    if module.check_mode:
        module.exit_json(**result)
    files0 = ''
    fcount = 0
    original_dir = os.getcwd()
    os.chdir(module.params['chdir'])
    for root, dirs, files in os.walk('.'):
        for d in dirs:
            path = os.path.join(root, d)
            files0 += (path + '\0')
        for f in files:
            fcount += 1
            path = os.path.join(root, f)
            files0 += (path + '\0')
    result['files'] = fcount

    cmd = [
        "/usr/bin/cpio",
        "--null",
        "-ov",
        "--format=newc",
        "--owner",
        "root:root"]
    with tempfile.TemporaryFile(mode='w+b', dir='/dev/shm') as outfile:
        process = subprocess.run(
            cmd,
            input=files0.encode("utf-8"),
            stdout=outfile,
            stderr=subprocess.PIPE
        )
        outfile.seek(0)
        os.chdir(original_dir)
        with gzip.open(module.params['dest'], 'wb', compresslevel=9) as gzfile:
            shutil.copyfileobj(outfile, gzfile)

    result['size'] = os.path.getsize(module.params['dest'])
    result['changed'] = True
    module.exit_json(**result)

def main():
    run_module()

if __name__ == '__main__':
    main()
