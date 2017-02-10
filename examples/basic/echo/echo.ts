// Copyright 2016 Marapongo, Inc. All rights reserved.

import * as mu from 'mu';

// Echo is a simple service that wraps an API gateway and exposes a single Say function.
class Echo extends mu.Service {
    constructor(
        // port is an optional port number for the API service.
        port: number = 80
    ) {
        new mu.APIGateway({
            api: this
            port: port
        });
    }

    // Say simply echoes back the given string.
    @mu.api()
    public Say(s: string): string {
        return s;
    }
}

