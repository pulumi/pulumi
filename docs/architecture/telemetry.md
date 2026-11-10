# Telemetry

## Overview

The pulumi CLI emits telemetry data to the service, capturing anonymous data about users interactions and performance data from the CLI.  This data is used to provide direction for the development of the CLI, and to further improve it.

## How it works

The CLI can send numbers, boolean, enums and fixed strings to the service. All data is validated on the service, and invalid data discarded. Any new fact that we're planning to send has to be added to the service first, otherwise the telemetry will be rejected.

## Adding new facts

As mentioned, new facts need to be added in two places, the CLI and the service.

### CLI

In the CLI, new facts are added to `pkg/telemetry/facts.go`.  The `enumFact`, `setFact`, `numberFact` or `boolFact` helper functions should be used to set the facts, and all facts should be constructed in this file, so we have a good overview of what's being sent.

### Service

In the service, the "schema" for the CLI telemetry is in [cmd/service/services/clitelemetry/schema.go#L50](https://github.com/pulumi/pulumi-service/blob/master/cmd/service/services/clitelemetry/schema.go#L50). Any fact that doesn't match the schema will automatically be rejected and discarded.  This includes a regex pattern for strings, which should be locked down to the tightest regex necessary.

## Dashboard

Now that we have the facts recorded, we also need to see them somewhere.  This is where metabase comes in. TODO: document this once we have this set up.
