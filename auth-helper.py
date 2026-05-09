#!/usr/bin/env python3
"""Verify Linux system user password
Usage: python3 auth-helper.py <username> <password>
Exits with 0 on success, 1 on failure
"""
import sys
import spwd
import crypt
import hashlib

def verify_password(username, password):
    try:
        sp = spwd.getspnam(username)
    except KeyError:
        return False
    
    shadow_pass = sp.sp_pwd
    if shadow_pass in ("", "!", "!!", "*", "NP", "LK", "!!"):
        return False
    
    # encrypted_password format: $id$salt$encrypted
    # Compare using crypt
    encrypted = crypt.crypt(password, shadow_pass)
    return encrypted == shadow_pass

if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("Usage: auth-helper.py <username> <password>", file=sys.stderr)
        sys.exit(2)
    
    success = verify_password(sys.argv[1], sys.argv[2])
    sys.exit(0 if success else 1)
