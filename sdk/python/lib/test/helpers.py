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

import logging
import functools
import pytest


def supress_unobserved_task_logging():
    """Suppresses logs about faulted unobserved tasks. This is similar to
    Python Pulumi user programs. See rationale in
    `sdk/python/cmd/pulumi-language-python-exec`.

    This scope of this setting necessarily bleeds beyond this test; it
    has to do so because the undesired logs appear after the entire
    `pytest` program terminates, not after a particular module
    terminates.

    """
    logging.getLogger("asyncio").setLevel(logging.CRITICAL)


# If calling code imports this module to use `raises`, it probably needs this.
supress_unobserved_task_logging()


def raises(exception_type):
    """Decorates a test by wrapping its body in `pytest.raises`."""

    def decorator(fn):
        @functools.wraps(fn)
        def wrapper(*args, **kwargs):
            with pytest.raises(exception_type):
                return fn(*args, **kwargs)

        return wrapper

    return decorator
