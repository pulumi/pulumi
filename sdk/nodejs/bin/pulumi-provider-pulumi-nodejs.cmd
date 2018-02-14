@echo off
SET REQUIRE_ROOT=%~dp0
SET REQUIRE_ROOT=%REQUIRE_ROOT:\=/%
REM We depend on a custom node build that has exposed some internal state
REM This node is downloaded and extracted via the EnsureCustomNode target
REM in the root build.proj
"%~dp0..\custom_node\node\node.exe" "%REQUIRE_ROOT%./cmd/dynamic-provider" %*
