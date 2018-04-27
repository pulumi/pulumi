@echo off

set NODE_PATH=%NODE_PATH%;%~dp0\v6.10.2
set PULUMI_RUN=./node_modules/@pulumi/pulumi/cmd/run
if not exist %PULUMI_RUN% (
    echo It looks like the Pulumi SDK has not been installed. Have you run npm install or yarn install?
    exit /b 1
)

node ./node_modules/@pulumi/pulumi/cmd/run %*
