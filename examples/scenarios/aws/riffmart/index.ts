// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as cluster from "./cluster";
import * as infra from "./infra";

export let cloud: boolean | undefined;

// Now create some useful variables that are used throughout the below logic.
let prefix = "riffmart-" + lumi.env();
let availabilityZone = aws.config.region + "a";

// First spin up the basic cluster infrastructure.
infra.new();

// TODO: the rest.

