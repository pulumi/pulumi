import argparse, json, os, re, subprocess, sys

parser = argparse.ArgumentParser(prog="wasm-size-breakdown")
parser.add_argument("binary")
parser.add_argument("--json", action="store_true")
args = parser.parse_args()

func_re = re.compile(r' - func\[\d+\] size=(\d+) <(.*)>')
segment_re = re.compile(r' - segment\[\d+\] memory=\d+ size=(\d+)')

result = subprocess.run(["wasm-objdump", "-x", args.binary], capture_output=True, text=True)
lines = result.stdout.splitlines()

def index(s, c, start=None):
    try:
        if start is None:
            return s.index(c)
        return s.index(c, start)
    except ValueError:
        return None

package_sizes = {}
code_size = 0
data_size = 0
for l in lines:
    m = func_re.match(l)
    if m is not None:
        (size, func) = m.group(1, 2)

        size = int(size)
        code_size += size

        # pkg.member
        # std_pkg.member
        # host.name_pkg.member

        first_underscore = func.find("_")
        first_dot = func.find(".")
        prev_dot = func.rfind(".", 0, first_underscore)
        next_dot = func.find(".", first_underscore if first_underscore > 0 else 0)

        end = next_dot
        if end == -1 or first_underscore != -1 and prev_dot != -1 and first_underscore == prev_dot + 1:
            end = first_dot
        end = end if end != -1 else len(func)

        package = func[:end].replace("_", "/")

        if package not in package_sizes:
            package_sizes[package] = size
        else:
            package_sizes[package] = package_sizes[package] + size
    else:
        m = segment_re.match(l)
        if m is not None:
            size = int(m.group(1))
            data_size += size

sorted_sizes = list(sorted(package_sizes.items(), key=lambda pair: pair[1], reverse=True))
if args.json:
    print(json.dumps({
        "code_size": code_size,
        "data_size": data_size,
        "packages": sorted_sizes,
        "total_size": code_size + data_size,
    }))
else:
    print(f"total size: {code_size + data_size}")
    print(f"code size: {code_size}")
    print(f"data size: {data_size}")
    print()

    sum = 0
    rows = [["package", "size", "percentage", "sum(size)", "sum(percentage)"]]
    for (package, size) in sorted_sizes:
        sum += size
        rows += [[package, size, f"{100.0 * size / code_size:.2f}%", sum, f"{100.0 * sum / code_size:.2f}%"]]

    column_widths = [0, 0, 0, 0, 0]
    for i in range(0, len(column_widths)):
        column_widths[i] = max(map(lambda row: len(str(row[i])), rows))

    for r in rows:
        for (i, c) in enumerate(r):
            pad = " " * (column_widths[i] - len(str(c)) + 2)
            print(f"{c}{pad}", end="")
        print()
