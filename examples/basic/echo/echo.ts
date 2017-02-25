// Copyright 2016 Pulumi, Inc. All rights reserved.

import * as coconutx from "coconutx";

// Echo is a simple service that wraps an API gateway and exposes a single Say function.
class Echo extends coconutx.Service {
    constructor(
        // port is an optional port number for the API service.
        port: number = 80
    ) {
        new coconutx.APIGateway({
            api: this
            port: port
        });
    }

    // Say simply echoes back the given string.
    @coconutx.api()
    public Say(s: string): string {
        return s;
    }
}

