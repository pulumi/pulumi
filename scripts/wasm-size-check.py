import argparse, os, sys

parser = argparse.ArgumentParser(prog="wasm-size-check")
parser.add_argument("binary")
parser.add_argument("gold")
parser.add_argument("-u", "--update", action="store_true")
args = parser.parse_args()

max_size = None
try:
    with open(args.gold, encoding="utf-8") as f:
        max_size = int(f.read().strip())
except (FileNotFoundError, ValueError):
    pass

def print_size_diff(actual):
    print(f"maximum size: {max_size}")
    print(f"actual size: {actual}")
    if max_size is not None:
        print(f"diff: {((actual - max_size) / max_size) * 100.0}%")

s = os.lstat(args.binary)
if max_size is None or s.st_size < (max_size * 0.95):
    if args.update:
        with open(args.gold, mode="w", encoding="utf-8") as f:
            f.write(str(s.st_size))
        sys.exit(0)

    print_size_diff(s.st_size)
    print(f"maximum size needs updating; run {sys.argv[0]} -u {args.binary} {args.gold}")
    sys.exit(1)

if s.st_size > (max_size * 1.01):
    print_size_diff(s.st_size)
    print(f"{args.binary} is too large; try refactoring to avoid adding dependencies")
    sys.exit(1)
