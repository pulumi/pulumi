@echo off

REM When making changes to this script, also update pulumi-resource-pulumi-python.cmd.

REM Save the first two arguments.
set "pulumi_policy_python_engine_address=%1"
set "pulumi_policy_python_program=%2"

REM Parse the -virtualenv command line argument.
set pulumi_runtime_python_virtualenv=
:parse
if "%~1"=="" goto endparse
if "%~1"=="-virtualenv" (
    REM Get the value as a fully-qualified path.
    set "pulumi_runtime_python_virtualenv=%~f2"
    goto endparse
)
shift /1
goto parse
:endparse

if defined pulumi_runtime_python_virtualenv (
    REM If python exists in the virtual environment, set PATH and run it.
    if exist "%pulumi_runtime_python_virtualenv%\Scripts\python.exe" (
        REM Update PATH and unset PYTHONHOME.
        set "PATH=%pulumi_runtime_python_virtualenv%\Scripts;%PATH%"
        set PYTHONHOME=

        REM Run python from the virtual environment.
        "%pulumi_runtime_python_virtualenv%\Scripts\python.exe" -u -m pulumi.policy %pulumi_policy_python_engine_address% %pulumi_policy_python_program%
        exit /B
    ) else (
        echo "%pulumi_runtime_python_virtualenv%" doesn't appear to be a virtual environment
        exit 1
    )
) else (
    REM Otherwise, just run python. We use `python` instead of `python3` because Windows
    REM Python installers install only `python.exe` by default.
    @python -u -m pulumi.policy %pulumi_policy_python_engine_address% %pulumi_policy_python_program%
)
