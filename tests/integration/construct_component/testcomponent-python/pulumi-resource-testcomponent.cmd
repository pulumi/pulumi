@echo off
setlocal
set SCRIPT_DIR=%~dp0
@cd "%SCRIPT_DIR%\..\..\..\..\sdk\python"
@pipenv run python "%SCRIPT_DIR%/testcomponent.py" %*
