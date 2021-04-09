@echo off
setlocal
set SCRIPT_DIR=%~dp0
@cd ..\..\..\..\sdk\python
@pipenv run python "%SCRIPT_DIR%/testcomponent.py" %*
