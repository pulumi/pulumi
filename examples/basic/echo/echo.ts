// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as lumix from "lumix";

// Echo is a simple service that wraps an API gateway and exposes a single Say function.
class Echo extends lumix.Service {
    constructor(
        // port is an optional port number for the API service.
        port: number = 80
    ) {
        new lumix.APIGateway({
            api: this
            port: port
        });
    }

    // Say simply echoes back the given string.
    @lumix.api()
    public Say(s: string): string {
        return s;
    }
}

