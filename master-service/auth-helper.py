#!/usr/bin/env python3
"""Verify a Linux system user password from a shadow file.

Set SYSTEM_SHADOW_FILE=/host/etc/shadow when the service runs in a container
and needs to authenticate against the host OS accounts.
"""
import crypt
import os
import sys


def verify_password(username, password):
    shadow_file = os.environ.get("SYSTEM_SHADOW_FILE", "/etc/shadow")
    shadow_pass = None

    try:
        with open(shadow_file, "r", encoding="utf-8") as f:
            for line in f:
                parts = line.rstrip("\n").split(":")
                if len(parts) >= 2 and parts[0] == username:
                    shadow_pass = parts[1]
                    break
    except OSError:
        return False

    if shadow_pass is None:
        return False

    if shadow_pass in ("", "!", "!!", "*", "NP", "LK"):
        return False

    encrypted = crypt.crypt(password, shadow_pass)
    return encrypted == shadow_pass


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("Usage: auth-helper.py <username> <password>", file=sys.stderr)
        sys.exit(2)

    success = verify_password(sys.argv[1], sys.argv[2])
    sys.exit(0 if success else 1)
