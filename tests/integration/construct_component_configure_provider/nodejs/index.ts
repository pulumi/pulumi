// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

import * as metaprovider from "@pulumi/metaprovider";
import * as pulumi from "@pulumi/pulumi";
import * as tls from "@pulumi/tls";

export = async () => {
  const config = new pulumi.Config();
  const proxy = config.require("proxy");
  const configurer = new metaprovider.Configurer("configurer", {
    tlsProxy: proxy
  });
  const key = new tls.PrivateKey("my-private-key", {
    algorithm: "ECDSA",
    ecdsaCurve: "P384",
  }, {
    provider: await configurer.tlsProvider()
  });
  const keyAlgo = key.algorithm;
  return {
    keyAlgo: key.algorithm
  }
};
