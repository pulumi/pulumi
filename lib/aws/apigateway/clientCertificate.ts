// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as cloudformation from "../cloudformation";

// The ClientCertificate resource creates a client certificate that Amazon API Gateway (API Gateway)
// uses to configure client-side SSL authentication for sending requests to the integration endpoint.
export class ClientCertificate extends cloudformation.Resource implements ClientCertificateProperties {
    public description?: string;

    constructor(name: string, args: ClientCertificateProperties) {
        super({
            name: name,
            resource: "AWS::ApiGateway::ClientCertificate",
        });
        this.description = args.description;
    }
}

export interface ClientCertificateProperties {
    // description is a description of the client certificate.
    description?: string;
}

