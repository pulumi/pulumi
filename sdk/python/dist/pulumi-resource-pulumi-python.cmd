@echo off

if defined PULUMI_RUNTIME_VIRTUALENV (
    REM If python exists in the virtual environment, set PATH and run it.
    if exist "%PULUMI_RUNTIME_VIRTUALENV%\Scripts\python.exe" (
        REM Update PATH and unset PYTHONHOME.
        set "PATH=%PULUMI_RUNTIME_VIRTUALENV%\Scripts;%PATH%"
        set PYTHONHOME=

        REM Run python from the virtual environment.
        "%PULUMI_RUNTIME_VIRTUALENV%\Scripts\python.exe" -u -m pulumi.dynamic %*
        exit /B
    ) else (
        echo "%PULUMI_RUNTIME_VIRTUALENV%" doesn't appear to be a virtual environment
        exit 1
    )
) else (
    REM Otherwise, just run python. We use `python` instead of `python3` because Windows
    REM Python installers install only `python.exe` by default.
    @python -u -m pulumi.dynamic %*
)
