@echo off

REM Get this script's directory without a trailing backslash. %%~dpI leaves a trailing backslash,
REM which on Windows escapes the closing quote in the `go run` argument below and mangles the path;
REM the "." plus %%~fI resolves it away.
for %%I in ("%~dp0.") do set SCRIPT=%%~fI

REM Run the Go program in the script's directory
go run "%SCRIPT%"
