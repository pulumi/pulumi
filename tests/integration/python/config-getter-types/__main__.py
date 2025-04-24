# Copyright 2024, Pulumi Corporation.  All rights reserved.

"""An program that type checks config getters"""

from typing import Optional
import pulumi

c = pulumi.Config()

a1 = c.get("foo")
a2 = c.get("foo", default="bar")
a3 = c.get("foo", default=None)

b1: Optional[int] = c.get_int("foo_int")
b2: int = c.get_int("foo_int", default=3)
b3: Optional[int] = c.get_int("foo_int", default=None)

c1: Optional[pulumi.Output[str]] = c.get_secret("foo_secret")
c2 = c.require("foo")
c3: int = c.require_int("foo_int")
c4: pulumi.Output[int] = c.require_secret_int("foo_secret")
