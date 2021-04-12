@echo off
setlocal
set SCRIPT_DIR=%~dp0
@%PULUMI_RUNTIME_VIRTUALENV%\Scripts\python.exe "%SCRIPT_DIR%/testcomponent.py" %*
