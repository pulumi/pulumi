// Copyright 2016-2021, Pulumi Corporation.  All rights reserved.

import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();

config.get("plainstr1");
config.require("plainstr2");
config.getSecret("plainstr3");
config.requireSecret("plainstr4");

config.getBoolean("plainbool1");
config.requireBoolean("plainbool2");
config.getSecretBoolean("plainbool3");
config.requireSecretBoolean("plainbool4");

config.getNumber("plainnum1");
config.requireNumber("plainnum2");
config.getSecretNumber("plainnum3");
config.requireSecretNumber("plainnum4");

config.getObject("plainobj1");
config.requireObject("plainobj2");
config.getSecretObject("plainobj3");
config.requireSecretObject("plainobj4");

config.get("str1");
config.require("str2");
config.getSecret("str3");
config.requireSecret("str4");

config.getBoolean("bool1");
config.requireBoolean("bool2");
config.getSecretBoolean("bool3");
config.requireSecretBoolean("bool4");

config.getNumber("num1");
config.requireNumber("num2");
config.getSecretNumber("num3");
config.requireSecretNumber("num4");

config.getObject("obj1");
config.requireObject("obj2");
config.getSecretObject("obj3");
config.requireSecretObject("obj4");

config.getObject("parent1");
config.requireObject("parent2");
config.getSecretObject("parent1");
config.requireSecretObject("parent2");

config.getObject("names1");
config.requireObject("names2");
config.getSecretObject("names1");
config.requireSecretObject("names2");
