echo %APPVEYOR_REPO_TAG_NAME%|findstr "^v[0-9]" >Nul 2>&1
if %errorlevel% == 0 ( 
dotnet msbuild /t:ReleaseProcess /v:Detailed build.proj
) else (
exit 0
)