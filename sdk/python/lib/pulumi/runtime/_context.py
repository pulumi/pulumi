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

"""Helpers for propagating runtime context across thread boundaries."""

from contextvars import copy_context

from opentelemetry import context as otel_context


def wrap_with_context(fn):
    """Wrap a callable so it runs with the current Python and OTel contexts.

    Use this when passing callables to run_in_executor, since thread pool
    threads do not inherit context from the calling thread.
    """
    python_ctx = copy_context()
    otel_ctx = otel_context.get_current()

    def run(*args, **kwargs):
        token = otel_context.attach(otel_ctx)
        try:
            return fn(*args, **kwargs)
        finally:
            otel_context.detach(token)

    def wrapper(*args, **kwargs):
        return python_ctx.run(run, *args, **kwargs)

    return wrapper
