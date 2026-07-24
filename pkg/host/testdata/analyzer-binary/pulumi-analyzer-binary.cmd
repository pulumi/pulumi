@echo off

REM Get this script's directory without a trailing backslash.
for %%I in ("%~dp0.") do set SCRIPT=%%~fI

REM Run the Go program in the script's directory
go run "%SCRIPT%"
