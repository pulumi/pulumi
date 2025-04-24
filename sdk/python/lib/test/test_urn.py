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


from pulumi import urn as urn_util


def test_parse_urn_with_name():
    res = urn_util._parse_urn(
        "urn:pulumi:stack::project::pulumi:providers:aws::default_4_13_0"
    )
    assert res.urn_name == "default_4_13_0"
    assert res.typ == "pulumi:providers:aws"
    assert res.pkg_name == "pulumi"
    assert res.mod_name == "providers"
    assert res.typ_name == "aws"


def test_parse_urn_without_name():
    res = urn_util._parse_urn("urn:pulumi:stack::project::pulumi:providers:aws")
    assert res.urn_name == ""
    assert res.typ == "pulumi:providers:aws"
    assert res.pkg_name == "pulumi"
    assert res.mod_name == "providers"
    assert res.typ_name == "aws"
