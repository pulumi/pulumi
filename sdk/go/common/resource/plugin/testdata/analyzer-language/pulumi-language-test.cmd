@echo off

REM Get the absolute path to this script
for %%I in ("%~f0") do set SCRIPT=%%~dpI

REM Run the Go program in the script's directory
go run "%SCRIPT%"
