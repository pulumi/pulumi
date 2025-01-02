@echo off

REM If PULUMI_RUNTIME_VIRTUALENV is not set, try to set it based on the PULUMI_RUNTIME_TOOLCHAIN.
if not defined PULUMI_RUNTIME_VIRTUALENV (
    if defined PULUMI_RUNTIME_TOOLCHAIN (
        if "%PULUMI_RUNTIME_TOOLCHAIN%"=="uv" (
            set PULUMI_RUNTIME_VIRTUALENV=.venv
        ) else if "%PULUMI_RUNTIME_TOOLCHAIN%"=="poetry" (
            for /f "tokens=*" %%i in ('poetry env info --path') do set PULUMI_RUNTIME_VIRTUALENV=%%i
        )
    )
)

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
        echo The 'virtualenv' option in Pulumi.yaml is set to %PULUMI_RUNTIME_VIRTUALENV%, but %PULUMI_RUNTIME_VIRTUALENV% doesn't appear to be a virtual environment. 1>&2
        echo Run the following commands to create the virtual environment and install dependencies into it: 1>&2
        echo     1. python -m venv %PULUMI_RUNTIME_VIRTUALENV% 1>&2
        echo     2. %PULUMI_RUNTIME_VIRTUALENV%\Scripts\python.exe -m pip install --upgrade pip setuptools wheel 1>&2
        echo     3. %PULUMI_RUNTIME_VIRTUALENV%\Scripts\python.exe -m pip install -r %cd%\requirements.txt 1>&2
        echo For more information see: https://www.pulumi.com/docs/intro/languages/python/#virtual-environments 1>&2
        exit 1
    )
) else (
    if defined PULUMI_PYTHON_CMD (
        REM If PULUMI_PYTHON_CMD is defined, run it.
        "%PULUMI_PYTHON_CMD%" -u -m pulumi.dynamic %*
    ) else (
        REM Otherwise, just run python. We use `python` instead of `python3` because Windows
        REM Python installers install only `python.exe` by default.
        @python -u -m pulumi.dynamic %*
    )
)
