// Copyright 2016 Marapongo, Inc. All rights reserved.

module echo
import mu

// Echo is a simple service that wraps an API gateway and exposes a single Say function.
service Echo {
    ctor() {
        new mu.APIGateway {
            api = this
            impl = "./mu.ts"
            port = this.properties.port
        }
    }

    interface {
        // Say simply echoes back the given string.
        Say(s: string): string
    }

    properties {
        // port is an optional port number for the API service.
        optional port: number = 80
    }
}

