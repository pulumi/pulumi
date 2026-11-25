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

import unittest

from pulumi.runtime.resource import _get_stack_trace_and_source_position


class StackTraceMetadataTests(unittest.TestCase):
    def test_full_capture_starts_with_caller(self):
        stack, _ = _get_stack_trace_and_source_position()
        self.assertEqual(stack.frames[0].module, __name__)
        self.assertEqual(stack.frames[0].origin, "")
        self.assertFalse(
            any(frame.module == "pulumi.runtime.resource" for frame in stack.frames)
        )
        self.assertTrue(any(frame.origin == "runtime" for frame in stack.frames))

    def test_registration_position_is_the_caller_of_the_concrete_constructor(self):
        class Resource:
            def __init__(self):
                self.trace, self.position = _get_stack_trace_and_source_position()

        class CustomResource(Resource):
            def __init__(self):
                super().__init__()

        class Bucket(CustomResource):
            def __init__(self):
                super().__init__()

        bucket = Bucket()
        self.assertTrue(bucket.trace.frames[3].HasField("pc"))
        self.assertEqual(bucket.position, bucket.trace.frames[3].pc)
        self.assertGreater(bucket.position.line, bucket.trace.frames[2].pc.line)

    def test_core_sdk_provider_sdk_and_launcher_are_distinct(self):
        for module, marker, origin in [
            ("pulumi.provider.server", False, "pulumi"),
            ("pulumi_aws.s3.bucket", False, ""),
            ("pulumi_helpers", False, ""),
            ("__main__", True, "pulumi"),
            ("__main__", False, ""),
        ]:
            with self.subTest(module=module, launcher=marker):
                namespace = {
                    "__name__": module,
                    "__pulumi_runtime__": marker,
                    "capture": _get_stack_trace_and_source_position,
                }
                exec(
                    compile("result, _ = capture()", "/example/module.py", "exec"),
                    namespace,
                )
                frame = namespace["result"].frames[0]
                self.assertEqual(frame.module, module)
                self.assertEqual(frame.origin, origin)
