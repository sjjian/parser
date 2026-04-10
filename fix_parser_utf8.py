#!/usr/bin/env python3
"""Fix invalid UTF-8 bytes in goyacc-generated parser files."""
import sys

for fname in sys.argv[1:]:
    with open(fname, 'rb') as f:
        data = f.read()
    try:
        data.decode('utf-8')
    except UnicodeDecodeError as e:
        fixed = data[:e.start] + b'\n'
        with open(fname, 'wb') as f:
            f.write(fixed)
        print(f'Fixed UTF-8 in {fname}')
