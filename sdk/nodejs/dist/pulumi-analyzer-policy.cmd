@echo off
setlocal
REM FIXME: bun
for /f "delims=" %%i in ('bun -e "console.log(require.resolve('@pulumi/pulumi/cmd/run-policy-pack'))"') do set PULUMI_RUN_SCRIPT_PATH=%%i
if DEFINED PULUMI_RUN_SCRIPT_PATH (
   REM FIXME: bun
   @bun "%PULUMI_RUN_SCRIPT_PATH%" %*
)
