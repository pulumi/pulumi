@echo off
REM We depend on a custom node build that has exposed some internal state
REM This node is downloaded and extracted via the EnsureCustomNode target
REM in the root build.proj
"%~dp0..\custom_node\node\node.exe" "./node_modules/@pulumi/pulumi/cmd/run" %*
