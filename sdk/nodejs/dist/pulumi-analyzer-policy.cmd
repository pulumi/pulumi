@echo off
setlocal
for /f "delims=" %%i in ('node -e "console.log(require.resolve('@pulumi/pulumi/cmd/run-policy-pack'))"') do set PULUMI_RUN_SCRIPT_PATH=%%i
if DEFINED PULUMI_RUN_SCRIPT_PATH (
   @node "%PULUMI_RUN_SCRIPT_PATH%" %*
)
