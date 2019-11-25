param($installPath, $toolsPath, $package, $project)

if($project.Object.SupportsPackageDependencyResolution)
{
    if($project.Object.SupportsPackageDependencyResolution())
    {
        # Do not install analyzers via install.ps1, instead let the project system handle it.
        return
    }
}

$analyzersPaths = Join-Path (Join-Path (Split-Path -Path $toolsPath -Parent) "analyzers") * -Resolve

foreach($analyzersPath in $analyzersPaths)
{
    if (Test-Path $analyzersPath)
    {
        # Install the language agnostic analyzers.
        foreach ($analyzerFilePath in Get-ChildItem -Path "$analyzersPath\*.dll" -Exclude *.resources.dll)
        {
            if($project.Object.AnalyzerReferences)
            {
                $project.Object.AnalyzerReferences.Add($analyzerFilePath.FullName)
            }
        }
    }
}

# $project.Type gives the language name like (C# or VB.NET)
$languageFolder = ""
if($project.Type -eq "C#")
{
    $languageFolder = "cs"
}
if($project.Type -eq "VB.NET")
{
    $languageFolder = "vb"
}
if($languageFolder -eq "")
{
    return
}

foreach($analyzersPath in $analyzersPaths)
{
    # Install language specific analyzers.
    $languageAnalyzersPath = join-path $analyzersPath $languageFolder
    if (Test-Path $languageAnalyzersPath)
    {
        foreach ($analyzerFilePath in Get-ChildItem -Path "$languageAnalyzersPath\*.dll" -Exclude *.resources.dll)
        {
            if($project.Object.AnalyzerReferences)
            {
                $project.Object.AnalyzerReferences.Add($analyzerFilePath.FullName)
            }
        }
    }
}