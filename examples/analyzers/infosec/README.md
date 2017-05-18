# examples/analyzers/infosec

This is an example analyzer that enforces corporate security policy.

To use it, run the `build.sh` script, ensure the output is on your `PATH`, and add it to your project file:

    analyzers:
        - infosec/basic

Or, alternatively, simply run a deployment with it listed explicitly:

    lumi deploy <env> --analyzer=infosec/basic

