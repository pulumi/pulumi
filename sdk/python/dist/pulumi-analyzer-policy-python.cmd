@echo off
setlocal
REM We use `python` instead of `python3` because Windows Python installers
REM install only `python.exe` by default.
@python -u -m pulumi.policy %*