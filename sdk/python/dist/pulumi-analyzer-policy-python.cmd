@echo off

REM Parse the -virtualenv command line argument and populate `args` with all other arguments.
set pulumi_runtime_python_virtualenv=
set args=
:parse
if "%~1"=="" goto endparse
if "%~1"=="-virtualenv" (
    REM Get the value as a fully-qualified path.
    set "pulumi_runtime_python_virtualenv=%~f2"
    shift /1
) else (
    set args=%args% %~1
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
        "%pulumi_runtime_python_virtualenv%\Scripts\python.exe" -u -m pulumi.policy %args%
        exit /B
    ) else (
        echo The 'virtualenv' option in PulumiPolicy.yaml is set to %pulumi_runtime_python_virtualenv%, but %pulumi_runtime_python_virtualenv% doesn't appear to be a virtual environment. 1>&2
        echo Run the following commands to create the virtual environment and install dependencies into it: 1>&2
        echo     1. python -m venv %pulumi_runtime_python_virtualenv% 1>&2
        echo     2. %pulumi_runtime_python_virtualenv%\Scripts\python.exe -m pip install --upgrade pip setuptools wheel 1>&2
        echo     3. %pulumi_runtime_python_virtualenv%\Scripts\python.exe -m pip install -r %cd%\requirements.txt 1>&2
        echo For more information see: https://www.pulumi.com/docs/intro/languages/python/#virtual-environments 1>&2
        exit 1
    )
) else (
    if defined PULUMI_PYTHON_CMD (
        REM If PULUMI_PYTHON_CMD is defined, run it.
        "%PULUMI_PYTHON_CMD%" -u -m pulumi.policy %args%
    ) else (
        REM Otherwise, just run python. We use `python` instead of `python3` because Windows
        REM Python installers install only `python.exe` by default.
        @python -u -m pulumi.policy %args%
    )
)
