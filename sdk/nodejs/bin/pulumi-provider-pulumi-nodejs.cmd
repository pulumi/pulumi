@echo off
cd "%~dp0"

REM We depend on a custom node build that has exposed some internal state
REM This node is downloaded and extracted via the EnsureCustomNode target
REM in the root build.proj
REM
REM NOTE: we pass a dummy argument before the actual args because the
REM provider module expects to be invoked as `node path/to/provider args`,
REM but we are invoking it with `-e`.
"%~dp0\..\custom_node\node.exe" -e "require('./cmd/dynamic-provider');" dummy_argument %*
