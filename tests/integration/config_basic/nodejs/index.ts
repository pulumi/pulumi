// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

import * as assert from "assert";
import { Config } from "@pulumi/pulumi";

// Just test that basic config works.
const config = new Config("config_basic_js");

// This value is plaintext and doesn't require encryption.
const value = config.require("aConfigValue");
assert.equal(value, "this value is a value", "aConfigValue not the expected value");

// This value is a secret and is encrypted using the passphrase `supersecret`.
const secret = config.require("bEncryptedSecret");
assert.equal(secret, "this super secret is encrypted", "bEncryptedSecret not the expected value");
