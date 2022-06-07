# Copyright 2016-2021, Pulumi Corporation.
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

from collections import namedtuple


_UrnParts = namedtuple(
    "_UrnParts", ["urn_name", "typ", "pkg_name", "mod_name", "typ_name"]
)


def _parse_urn(urn: str) -> _UrnParts:
    try:
        urn_parts = urn.split("::")
        urn_name = urn_parts[3] if len(urn_parts) >= 4 else ""
        qualified_type = urn_parts[2]
        typ = qualified_type.split("$")[-1]
        typ_parts = typ.split(":")
        pkg_name = typ_parts[0]
        mod_name = typ_parts[1] if len(typ_parts) > 1 else ""
        typ_name = typ_parts[2] if len(typ_parts) > 2 else ""
        return _UrnParts(
            urn_name=urn_name,
            typ=typ,
            pkg_name=pkg_name,
            mod_name=mod_name,
            typ_name=typ_name,
        )
    except Exception as e:
        raise ValueError(f"Cannot parse URN: {urn}") from e
