@echo off
setlocal
REM FIXME: bun
for /f "delims=" %%i in ('bun -e "console.log(require.resolve('@pulumi/pulumi/cmd/dynamic-provider'))"') do set PULUMI_DYNAMIC_PROVIDER_SCRIPT_PATH=%%i
if DEFINED PULUMI_DYNAMIC_PROVIDER_SCRIPT_PATH (
   REM FIXME: bun
   @bun "%PULUMI_DYNAMIC_PROVIDER_SCRIPT_PATH%" %*
)
