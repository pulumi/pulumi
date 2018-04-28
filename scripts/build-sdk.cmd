@echo off
setlocal EnableDelayedExpansion

set s3_bucket_root=s3://eng.pulumi.com/
set s3_publish_folder=%s3_bucket_root%releases/sdk/
set s3_rootfolder_pulumi_windows=s3://eng.pulumi.com/releases/pulumi/windows/amd64/
set s3_rootfolder_pulumi_aws_windows=s3://eng.pulumi.com/releases/pulumi-aws/windows/amd64/
set s3_rootfolder_pulumi_azure_windows=s3://eng.pulumi.com/releases/pulumi-azure/windows/amd64/
set s3_rootfolder_pulumi_kubernetes_windows=s3://eng.pulumi.com/releases/pulumi-kubernetes/windows/amd64/
set s3_rootfolder_pulumi_cloud_windows=s3://eng.pulumi.com/releases/pulumi-cloud/windows/amd64/
set versionInput=
set refName=master

if [%1] NEQ [] (
    set versionInput=%1
) else (
    set file_date=%DATE:~-4,4%%DATE:~-7,2%%DATE:~-10,2%
    set file_time=%TIME:~0,2%%TIME:~3,2%%TIME:~6,2%
    set versionInput=!file_date!_!file_time!
)

if [%2] NEQ [] (
   set refName=%2
)

set sdkfilename=pulumi-%versionInput%-windows.x64.zip

echo Upon completion package: %sdkfilename% will be uploaded to S3 location: %s3_publish_folder%

where npm.cmd >nul 2>nul
if %ERRORLEVEL% NEQ 0 (   
        echo Please install npm ^(and add them to your %%PATH%%^) before running this script
        exit /B
)

where 7z.exe >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo Please install 7z ^(and add them to your %%PATH%%^) before running this script
    exit /B
)

where aws.exe >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo Please install AWS CLI ^(and add them to your %%PATH%%^) before running this script
    exit /B
)

set pulumi_folder=%TEMP%\pulumi_%random%
echo %pulumi_folder%

IF EXIST %pulumi_folder% (
    echo "Deleting existing Pulumi folder"
    rd /s /q %pulumi_folder%
)

echo Creating Temporary Pulumi folder
mkdir %pulumi_folder%

echo Switching to Temporary folder
pushd .
cd /d %pulumi_folder%

mkdir Pulumi
cd Pulumi

echo copying install.cmd from dist
copy "%~dp0\..\dist\install.cmd" .

call:processPackage pulumi .zip %s3_rootfolder_pulumi_windows% 
if %ERRORLEVEL% NEQ 0 (
    echo Failed to process Package for pulumi
    goto:eof
)
call:processPackage pulumi-aws .tgz %s3_rootfolder_pulumi_aws_windows% 
if %ERRORLEVEL% NEQ 0 (
    echo Failed to process Package for pulumi-aws
    goto:eof
)

call:processPackage pulumi-azure .tgz %s3_rootfolder_pulumi_azure_windows% 
if %ERRORLEVEL% NEQ 0 (
    echo Failed to process Package for pulumi-azure
    goto:eof
)

call:processPackage pulumi-kubernetes .tgz %s3_rootfolder_pulumi_kubernetes_windows% 
if %ERRORLEVEL% NEQ 0 (
    echo Failed to process Package for pulumi-kubernetes
    goto:eof
)

call:processPackage pulumi-cloud .tgz %s3_rootfolder_pulumi_cloud_windows% 
if %ERRORLEVEL% NEQ 0 (
    echo Failed to process Package for pulumi-cloud
    goto:eof
)

echo npm install pulumi and its components

call:npmInstall node_modules\pulumi

call:npmInstall node_modules\@pulumi\aws

call:npmInstall node_modules\@pulumi\azurerm

call:npmInstall node_modules\@pulumi\kubernetes

call:npmInstall node_modules\@pulumi\cloud

call:npmInstall node_modules\@pulumi\cloud-aws

cd ..

echo zip contents of the folder in a package

7z a -tzip %sdkfilename%
if %ERRORLEVEL% NEQ 0 (
    echo Failed to zip the contents of the folder to file: %sdkfilename%
    exit /b
)

echo Upload %sdkfilename% to aws s3 bucket %s3_publish_folder%
aws s3 cp --only-show-errors %sdkfilename% %s3_publish_folder%

if %ERRORLEVEL% NEQ 0 (
    echo Failed to upload package: %sdkfilename% to S3 Bucket: %s3_publish_folder%
    exit /b
)

echo Successfully created Windows Installation Package: %sdkfilename% and uploaded to S3 Bucket: %s3_publish_folder%

popd
echo Deleting Temporary folder
rd /s /q %pulumi_folder%
echo Success.
goto:eof

:processPackage
    @echo off
    echo Getting Packages for repo: %~1 pkg type: %~2
    setlocal

    set gitrepo=https://github.com/pulumi/%~1.git

    for /f "tokens=1" %%A IN ('git ls-remote -h -t %gitrepo% %refName%') DO set commit=%%A
    if %ERRORLEVEL% NEQ 0 (
        echo Failed to get last commit hash for %gitrepo%
        exit /b
    )
    set file=%commit%%~2%
    set s3-file=%~3%file%
    aws s3 cp --only-show-errors %s3-file% .
    if %ERRORLEVEL% NEQ 0 (
        echo Failed to get package: %s3-file% from AWS S3
        exit /b
    )
    echo unzipping %file%
    if %~2 NEQ .zip (            
        7z x %file% -tgzip -so | 7z x -ttar -si
    ) else (
        7z x %file%
    )

    if %ERRORLEVEL% NEQ 0 (
        echo Failed to unzip package: %file%
        exit /b
    ) else (
        echo deleting the package: %file%
        del /q %file%
    )
goto:eof

:npmInstall
    echo Installing Node modules from %~1
    pushd %~1
    CALL npm install --only=production
    if %ERRORLEVEL% NEQ 0 (
        echo npm install failed for Module: %~1
    )
    popd
goto:eof
