# Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import pulumi

config = pulumi.Config()

config.get("plainstr1")
config.require("plainstr2")
config.get_secret("plainstr3")
config.require_secret("plainstr4")

config.get_bool("plainbool1")
config.require_bool("plainbool2")
config.get_secret_bool("plainbool3")
config.require_secret_bool("plainbool4")

config.get_int("plainint1")
config.require_int("plainint2")
config.get_secret_int("plainint3")
config.require_secret_int("plainint4")

config.get_float("plainfloat1")
config.require_float("plainfloat2")
config.get_secret_float("plainfloat3")
config.require_secret_float("plainfloat4")

config.get_object("plainobj1")
config.require_object("plainobj2")
config.get_secret_object("plainobj3")
config.require_secret_object("plainobj4")

config.get("str1")
config.require("str2")
config.get_secret("str3")
config.require_secret("str4")

config.get_bool("bool1")
config.require_bool("bool2")
config.get_secret_bool("bool3")
config.require_secret_bool("bool4")

config.get_int("int1")
config.require_int("int2")
config.get_secret_int("int3")
config.require_secret_int("int4")

config.get_float("float1")
config.require_float("float2")
config.get_secret_float("float3")
config.require_secret_float("float4")

config.get_object("obj1")
config.require_object("obj2")
config.get_secret_object("obj3")
config.require_secret_object("obj4")

config.get_object("parent1")
config.require_object("parent2")
config.get_secret_object("parent1")
config.require_secret_object("parent2")

config.get_object("names1")
config.require_object("names2")
config.get_secret_object("names1")
config.require_secret_object("names2")
