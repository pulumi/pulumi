# Copyright 2026, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import struct
import sys

SECURITY_DIRECTORY = 4


def is_signed(path: str) -> bool:
    with open(path, "rb") as f:
        data = f.read()
    pe_offset = struct.unpack_from("<I", data, 0x3C)[0]
    magic = struct.unpack_from("<H", data, pe_offset + 24)[0]
    data_directories = pe_offset + 24 + (96 if magic == 0x10B else 112)
    _, size = struct.unpack_from("<II", data, data_directories + 8 * SECURITY_DIRECTORY)
    return size > 0


def main() -> int:
    paths = sys.argv[1:]
    if not paths:
        print("no binaries to check", file=sys.stderr)
        return 1
    unsigned = [p for p in paths if not is_signed(p)]
    for p in paths:
        print(f"{'unsigned' if p in unsigned else 'signed  '}  {p}")
    return 1 if unsigned else 0


if __name__ == "__main__":
    sys.exit(main())
