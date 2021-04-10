@echo off
setlocal
set SCRIPT_DIR=%~dp0
@%PULUMI_RUNTIME_VIRTUALENV%\bin\python.exe "%SCRIPT_DIR%/testcomponent.py" %*
