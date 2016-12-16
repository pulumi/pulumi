module xovnoc

// TODO: lambda code.
// TODO: that big nasty UserData shell script.
// TODO: possibly even refactor individual things into services (e.g., the networks).
// TODO: we probably need a ToString()-like thing for services (e.g., ARN/ID for most AWS ones).

service Rack {
    new() {
        security := new rackSecurity { this.properties }
        network := new rackNetwork { this.properties }
        logging := new rackLogging {
            this.properties
            role: security.logSubscriptionFilterRole
        }
        storage := new rackStorage { this.properties }
        services := new rackServices { this.properties}
        volumes := new rackVolumes {
            this.properties
            vpc: network.vpc
            subnets: private ? network.privateSubnets : subnets
        }
    }

    properties: rackSecurity & rackNetwork & rackLogging & rackStorage & rackServices & rackVolumes {
        // Development mode
        development: bool = false
    }
}

