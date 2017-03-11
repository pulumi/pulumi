# examples/analyzers/acmecorp-security

This is an example analyzer that enforces ACMECorp's corporate security policy.

To use it, run the `build.sh` script, ensure the output is on your `PATH`, and add it to your project file:

    analyzers:
        - acmecorp/security

Or, alternatively, simply run a deployment with it listed explicitly:

    coco deploy <env> --analyzer=acmecorp/security

