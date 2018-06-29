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
import six
from pulumi import CustomResource
from pulumi.runtime import Unknown


class Bucket(CustomResource):
    def __init__(self, name):
        CustomResource.__init__(self, "test:index:Bucket", name)

    def set_outputs(self, outputs):
        self.bucket = Unknown()
        self.stable = Unknown()
        if "stable" in outputs:
            self.bucket = outputs["stable"]
        if "bucket" in outputs:
            self.bucket = outputs["bucket"]


class BucketObject(CustomResource):
    def __init__(self, name, bucket=None):
        if not bucket:
            raise TypeError("bucket is required")

        if not isinstance(bucket, six.string_types):
            raise TypeError("bucket must be a string")

        CustomResource.__init__(self, "test:index:BucketObject", name, props={
            "bucket": bucket
        })

    def set_outputs(self, outputs):
        self.stabke = Unknown()
        self.bucket = Unknown()
        if "stable" in outputs:
            self.bucket = outputs["stable"]
        if "bucket" in outputs:
            self.bucket = outputs["bucket"]

bucket = Bucket("mybucket")
obj = BucketObject("mybucketobject", bucket=bucket.id)
