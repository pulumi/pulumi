@echo off
setlocal
set SCRIPT_DIR=%~dp0
@pipenv run python "%SCRIPT_DIR%/testcomponent.py" %*
