@set NODE_PATH=%NODE_PATH%;%~dp0.\v6.10.2
@pulumi-langhost-nodejs-node.exe ./node_modules/@pulumi/pulumi/cmd/run %*
