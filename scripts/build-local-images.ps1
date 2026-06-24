[CmdletBinding()]
param(
    [string]$ServerTag = "memoh-server:mimo-local",
    [string]$WebTag = "memoh-web:mimo-local",
    [string]$OutputDir = "dist/local-images",
    [string]$MirrorPrefix = "docker.1ms.run",
    [string]$Platform = "",
    [switch]$SkipSave,
    [switch]$KeepTempDockerfiles,
    [switch]$NoSyntaxFrontend
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host "==> $Message"
}

function Require-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required command not found: $Name"
    }
}

function Get-GitValue {
    param(
        [string[]]$Args,
        [string]$Fallback = ""
    )
    try {
        $result = (& git @Args 2>$null)
        if ($LASTEXITCODE -eq 0 -and $result) {
            return ($result | Select-Object -First 1).Trim()
        }
    } catch {
    }
    return $Fallback
}

function Convert-BaseImage {
    param(
        [string]$Image,
        [string]$Mirror
    )
    if ([string]::IsNullOrWhiteSpace($Mirror) -or $Image -eq "scratch") {
        return $Image
    }
    if ($Image.StartsWith("$Mirror/")) {
        return $Image
    }
    if ($Image.Contains("/")) {
        return "$Mirror/$Image"
    }
    return "$Mirror/library/$Image"
}

function Write-LocalDockerfile {
    param(
        [string]$SourcePath,
        [string]$TargetPath,
        [string]$Mirror,
        [bool]$DropSyntaxLine
    )

    $lines = Get-Content $SourcePath
    $out = New-Object System.Collections.Generic.List[string]

    foreach ($line in $lines) {
        if ($DropSyntaxLine -and $line -match '^\s*#\s*syntax=') {
            continue
        }
        if ($line -match '^(FROM(?:\s+--platform=\S+)?\s+)(\S+)(.*)$') {
            $prefix = $matches[1]
            $image = $matches[2]
            $suffix = $matches[3]
            $mapped = Convert-BaseImage -Image $image -Mirror $Mirror
            $out.Add("$prefix$mapped$suffix")
            continue
        }
        $out.Add($line)
    }

    [System.IO.File]::WriteAllLines($TargetPath, $out)
}

function Invoke-Docker {
    param([string[]]$Arguments)
    & docker @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Docker command failed: docker $($Arguments -join ' ')"
    }
}

function Get-ArchiveName {
    param([string]$ImageTag)
    $safe = ($ImageTag -replace '[:/\\]', '-')
    return "$safe.tar"
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repoRoot

Require-Command docker
Require-Command git

$version = Get-GitValue -Args @("describe", "--tags", "--always", "--dirty") -Fallback "dev"
$commitHash = Get-GitValue -Args @("rev-parse", "--short", "HEAD") -Fallback "unknown"
$buildTime = [DateTimeOffset]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")
$outDir = Join-Path $repoRoot $OutputDir
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("memoh-local-image-build-" + [guid]::NewGuid().ToString("N"))
$tempServerDockerfile = Join-Path $tempRoot "Dockerfile.server.local"
$tempWebDockerfile = Join-Path $tempRoot "Dockerfile.web.local"

New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null

try {
    Write-Step "Preparing temporary Dockerfiles"
    Write-LocalDockerfile -SourcePath (Join-Path $repoRoot "docker/Dockerfile.server") -TargetPath $tempServerDockerfile -Mirror $MirrorPrefix -DropSyntaxLine:$NoSyntaxFrontend
    Write-LocalDockerfile -SourcePath (Join-Path $repoRoot "docker/Dockerfile.web") -TargetPath $tempWebDockerfile -Mirror $MirrorPrefix -DropSyntaxLine:$NoSyntaxFrontend

    $serverArgs = @(
        "build",
        "-f", $tempServerDockerfile,
        "-t", $ServerTag,
        "--build-arg", "VERSION=$version",
        "--build-arg", "COMMIT_HASH=$commitHash",
        "--build-arg", "BUILD_TIME=$buildTime"
    )
    if (-not [string]::IsNullOrWhiteSpace($Platform)) {
        $serverArgs += @("--platform", $Platform)
    }
    $serverArgs += "."

    $webArgs = @(
        "build",
        "-f", $tempWebDockerfile,
        "-t", $WebTag
    )
    if (-not [string]::IsNullOrWhiteSpace($Platform)) {
        $webArgs += @("--platform", $Platform)
    }
    $webArgs += "."

    Write-Step "Building server image $ServerTag"
    Invoke-Docker -Arguments $serverArgs

    Write-Step "Building web image $WebTag"
    Invoke-Docker -Arguments $webArgs

    if (-not $SkipSave) {
        Write-Step "Exporting tarballs to $outDir"
        New-Item -ItemType Directory -Force -Path $outDir | Out-Null

        $serverTar = Join-Path $outDir (Get-ArchiveName -ImageTag $ServerTag)
        $webTar = Join-Path $outDir (Get-ArchiveName -ImageTag $WebTag)

        Invoke-Docker -Arguments @("save", "-o", $serverTar, $ServerTag)
        Invoke-Docker -Arguments @("save", "-o", $webTar, $WebTag)

        $overridePath = Join-Path $outDir "docker-compose.local-images.yml"
        @"
services:
  migrate:
    image: $ServerTag
  server:
    image: $ServerTag
  web:
    image: $WebTag
"@ | Set-Content -Path $overridePath -NoNewline

        Write-Host ""
        Write-Host "Artifacts:"
        Write-Host "  $serverTar"
        Write-Host "  $webTar"
        Write-Host "  $overridePath"
    }

    Write-Host ""
    Write-Host "Build complete."
    Write-Host "Server image: $ServerTag"
    Write-Host "Web image:    $WebTag"
    Write-Host "Version:      $version"
    Write-Host "Commit:       $commitHash"
} finally {
    if (-not $KeepTempDockerfiles -and (Test-Path $tempRoot)) {
        Remove-Item -Recurse -Force $tempRoot
    }
}
