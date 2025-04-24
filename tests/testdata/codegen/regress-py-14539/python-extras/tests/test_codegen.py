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

import asyncio
import pytest

import pulumi

from pulumi_gcp.compute.instance import Instance, InstanceArgs
from pulumi_gcp.compute.instancebootdisk import InstanceBootDiskArgs
from pulumi_gcp.compute.instancebootdiskinitializeparams import InstanceBootDiskInitializeParamsArgs


@pytest.fixture
def my_mocks():
    old_settings = pulumi.runtime.settings.SETTINGS
    try:
        mocks = MyMocks()
        pulumi.runtime.mocks.set_mocks(mocks)
        yield mocks
    finally:
        pulumi.runtime.settings.configure(old_settings)


class MyMocks(pulumi.runtime.Mocks):
    def call(self, args):
        return {}
    def new_resource(self, args):
        assert args.inputs == {
            "bootDisk": {
                "initializeParams": {
                    "imageName": "debian-cloud/debian-11",
                },
            },
        }
        return "foo", args.inputs


def identity(x):
    return x


def wrap_output(x):
    return pulumi.Output.from_input(x)


def wrap_future(x):
    future = asyncio.Future()
    future.set_result(x)
    return future


@pytest.mark.parametrize("boot_disk_wrap", [identity, wrap_output, wrap_future])
@pytest.mark.parametrize("initialize_params_wrap", [identity, wrap_output, wrap_future])
@pytest.mark.parametrize("image_name_wrap", [identity, wrap_output, wrap_future])
@pytest.mark.parametrize("initialize_params", ["initialize_params", "initializeParams"])
@pytest.mark.parametrize("image_name", ["image_name", "imageName"])
@pulumi.runtime.test
def test_dict(
    my_mocks,
    boot_disk_wrap,
    initialize_params_wrap,
    image_name_wrap,
    initialize_params,
    image_name,
):
    Instance(
        "instance-1",
        boot_disk=boot_disk_wrap({
            initialize_params: initialize_params_wrap({
                image_name: image_name_wrap("debian-cloud/debian-11"),
            }),
        }),
    )


@pytest.mark.parametrize("boot_disk_wrap", [identity, wrap_output, wrap_future])
@pytest.mark.parametrize("initialize_params_wrap", [identity, wrap_output, wrap_future])
@pytest.mark.parametrize("image_name_wrap", [identity, wrap_output, wrap_future])
@pulumi.runtime.test
def test_input_types(
    my_mocks,
    boot_disk_wrap,
    initialize_params_wrap,
    image_name_wrap,
):
    Instance(
        "instance-1",
        boot_disk=boot_disk_wrap(InstanceBootDiskArgs(
            initialize_params=initialize_params_wrap(InstanceBootDiskInitializeParamsArgs(
                image_name=image_name_wrap("debian-cloud/debian-11"),
            )),
        )),
    )

    Instance(
        "instance-2",
        InstanceArgs(
            boot_disk=boot_disk_wrap(InstanceBootDiskArgs(
                initialize_params=initialize_params_wrap(InstanceBootDiskInitializeParamsArgs(
                    image_name=image_name_wrap("debian-cloud/debian-11"),
                )),
            )),
        ),
    )

@pulumi.runtime.test
def test_mutate_input_types(my_mocks):
    initialize_params_args = InstanceBootDiskInitializeParamsArgs()
    initialize_params_args.image_name = "debian-cloud/debian-11"

    boot_disk_args = InstanceBootDiskArgs(initialize_params="bad-initialize-params")
    boot_disk_args.initialize_params = initialize_params_args
    Instance("instance-1", boot_disk=boot_disk_args)

    instance_args = InstanceArgs(boot_disk="bad-boot-disk")
    instance_args.boot_disk = boot_disk_args

    Instance(
        "instance-2",
        instance_args,
    )
