# Copyright 2016-2023, Pulumi Corporation.
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


import pytest


@pytest.mark.parametrize(
    "key,default",
    [
        ("string", None),
        ("bar", "baz"),
        ("doesnt-exist", None),
    ],
)
@pytest.mark.asyncio
async def test_config_with_defaults(key, default, mock_config, config_settings):
    expected = config_settings.get(f"test-config:{key}", default)

    assert mock_config.get(key, default) == expected

    result = mock_config.get_secret(key, default)
    if result is None:
        assert result == expected
    else:
        actual = await result.future()
        assert actual == expected
