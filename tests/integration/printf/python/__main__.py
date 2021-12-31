# Copyright 2021, Pulumi Corporation.  All rights reserved.

import sys

def main():
    for i in range(0, 10):
        print(f'Line {i}')
        print(f'Errln {i+10}', file=sys.stderr)
    print(f'Line 10', end='')
    print(f'Errln 20', end='', file=sys.stderr)
    return None

if __name__ == "__main__":
    main()
