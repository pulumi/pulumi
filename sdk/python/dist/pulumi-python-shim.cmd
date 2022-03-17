@echo off
REM Used to wrap python interpreters affected by https://github.com/golang/go/issues/42919
REM Expect first argument is the path to the interpreter to invoke. The rest of the args
REM are passed to the interpreter without modification

python %*
