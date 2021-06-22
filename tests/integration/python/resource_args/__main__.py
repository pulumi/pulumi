# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import pulumi
from pulumi_example import Foo, FooArgs, Provider


class MyMocks(pulumi.runtime.Mocks):
    resources = {}

    def call(self, args: pulumi.runtime.MockCallArgs):
        return {}

    def new_resource(self, args: pulumi.runtime.MockResourceArgs):
        if args.name == "f1":
            assert args.inputs == {"first": 1, "second": "second", "third": "third"}
        elif args.name == "f2":
            assert args.inputs == {"args": "args", "first": 2, "second": "s", "third": "t"}
        elif args.name == "f3":
            assert len(args.inputs) == 0
            assert args.provider.endswith("f3provider_id")
        elif args.name == "f4":
            assert args.inputs == {"args": "hi"}
        elif args.name == "f5":
            assert args.inputs == {"first": 100, "second": "200", "third": "300"}
            assert args.provider.endswith("f5provider_id")
        elif args.name == "a1":
            assert args.inputs == {"first": 10, "second": "asecond", "third": "athird"}
        elif args.name == "a2":
            assert args.inputs == {"first": 42, "second": "2nd", "third": "3rd"}
        elif args.name == "a3":
            assert args.inputs == {"args": "someargs", "first": 50, "second": "2", "third": "3"}
        elif args.name == "a4":
            assert args.inputs == {"first": 11, "second": "12", "third": "13"}
            assert args.provider.endswith("a4provider_id")
        return [args.name + '_id', args.inputs]


pulumi.runtime.set_mocks(MyMocks())

Foo("f1", first=1, second="second", third="third")
Foo("f2", None, "args", 2, "s", "t")
Foo("f3", pulumi.ResourceOptions(provider=Provider("f3provider")))
Foo("f4", args="hi")
Foo("f5", first=100, second="200", third="300", opts=pulumi.ResourceOptions(provider=Provider("f5provider")))
Foo("a1", FooArgs(first=10, second="asecond", third="athird"))
Foo("a2", args=FooArgs(first=42, second="2nd", third="3rd"))
Foo("a3", args=FooArgs(args="someargs", first=50, second="2", third="3"))
Foo("a4", FooArgs(first=11, second="12", third="13"), pulumi.ResourceOptions(provider=Provider("a4provider")))
