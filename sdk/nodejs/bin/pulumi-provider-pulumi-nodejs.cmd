@echo off
cd "%~dp0"

REM We depend on a custom node build that has exposed some internal state
REM This node is downloaded and extracted via the EnsureCustomNode target
REM in the root build.proj
"%~dp0\..\custom_node\node.exe" -e "require('./cmd/dynamic-provider');" %*
