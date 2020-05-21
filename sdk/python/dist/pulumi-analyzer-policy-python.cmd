@echo off

:: Save the first two arguments.
set "pulumi_policy_python_engine_address=%1"
set "pulumi_policy_python_program=%2"

:: Parse the -virtualenv command line argument.
set pulumi_policy_python_virtualenv=
:parse
if "%~1"=="" goto endparse
if "%~1"=="-virtualenv" (
    :: Get the value as a fully-qualified path.
    set "pulumi_policy_python_virtualenv=%~f2"
    goto endparse
)
shift /1
goto parse
:endparse

if defined pulumi_policy_python_virtualenv (
    :: If python exists in the virtual environment, set PATH and run it.
    if exist "%pulumi_policy_python_virtualenv%\Scripts\python.exe" (
        :: Update PATH and unset PYTHONHOME.
        set "PATH=%pulumi_policy_python_virtualenv%\Scripts;%PATH%"
        set PYTHONHOME=

        :: Run python from the virtual environment.
        "%pulumi_policy_python_virtualenv%\Scripts\python.exe" -u -m pulumi.policy %pulumi_policy_python_engine_address% %pulumi_policy_python_program%
        exit /B
    ) else (
        echo "%pulumi_policy_python_virtualenv%" doesn't appear to be a virtual environment
        exit 1
    )
) else (
    :: Otherwise, just run python. We use `python` instead of `python3` because Windows
    :: Python installers install only `python.exe` by default.
    @python -u -m pulumi.policy %pulumi_policy_python_engine_address% %pulumi_policy_python_program%
)
