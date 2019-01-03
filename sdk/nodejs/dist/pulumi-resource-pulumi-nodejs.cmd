@echo off
setlocal
for /f "delims=" %%i in ('node -e "console.log(require.resolve('@pulumi/pulumi/cmd/dynamic-provider'))"') do set PULUMI_DYNAMIC_PROVIDER_SCRIPT_PATH=%%i
if DEFINED PULUMI_DYNAMIC_PROVIDER_SCRIPT_PATH (
   @node "%PULUMI_DYNAMIC_PROVIDER_SCRIPT_PATH%" %*
)
