# Copyright 2016-2018, Pulumi Corporation.
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

"""
The Pulumi SDK test package for testing with mocks.
"""

# The mocks tests are in their own `test_with_mocks` package so they can be run separately
# from other tests in the `test` package. Otherwise, if the mocks tests were in the same
# package as other tests, the code that initializes the mocks and test resources would run
# during test discovery and impact the behavior of other tests due to modifying global state.
