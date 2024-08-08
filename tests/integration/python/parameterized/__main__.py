# Copyright 2016-2024, Pulumi Corporation.  All rights reserved.

import pulumi_pkg as pkg

res1 = pkg.Random("res1", length=5)

res2 = pkg.do_echo("hello")

res3 = pkg.do_echo_output("hello")

res4 = pkg.Echo("echo")

res5 = res4.do_echo_method(echo="hello")

# Read on testprovider.Random just returns inputs back, so this works even though the resource
# doesn't exist in the state.
res6 = pkg.Random.get("res6", "banana")
