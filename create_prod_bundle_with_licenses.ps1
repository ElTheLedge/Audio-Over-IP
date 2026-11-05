# build_release.ps1
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ProjectRoot = (Get-Location).Path
$GoPackage = "AuOvIP"
$FrontendDir = Join-Path $ProjectRoot "frontend"
$OutputDir = Join-Path $ProjectRoot "THIRD_PARTY_LICENSES"
$ZipName = "build/bin/AudioOverIP_release-Windows_x64.zip"
$BinaryPath = Join-Path $ProjectRoot "build/bin/AudioOverIP.exe"

Write-Host "Cleaning old output..."
if (Test-Path $OutputDir) { Remove-Item $OutputDir -Recurse -Force }
if (Test-Path $ZipName) { Remove-Item $ZipName -Force }
New-Item -ItemType Directory -Path $OutputDir | Out-Null

##############################################
# Helper: Download GitHub license for exact tag
##############################################
function Get-GitHubVersionedLicense {
    param(
        [string]$Url,
        [string]$OutFile
    )

    # Match: github.com/<owner>/<repo>/blob/<tag>/<path>
    $regex = "github\.com/([^/]+)/([^/]+)/blob/([^/]+)/(.*)$"
    $match = [regex]::Match($Url, $regex)

    if (-not $match.Success) {
        return $false
    }

    $Owner = $match.Groups[1].Value
    $RepoName = $match.Groups[2].Value
    $Tag = $match.Groups[3].Value
    $FilePath = $match.Groups[4].Value

    $RawUrl = "https://raw.githubusercontent.com/$Owner/$RepoName/refs/tags/$Tag/$FilePath"

    try {
        Invoke-WebRequest -Uri $RawUrl -OutFile $OutFile -UseBasicParsing
        if ((Get-Content $OutFile -Raw).Length -gt 0) {
            return $true
        }
    }
    catch {
        return $false
    }

    return $false
}

##############################################
# 1. Collect Go dependency licenses
##############################################

Write-Host "Collecting Go dependency licenses..."

$GoLicenses = go-licenses report $GoPackage

foreach ($line in $GoLicenses) {
    $parts = $line -split ","
    $PkgName = $parts[0]
    $LicenseUrl = $parts[1]
    $LicenseType = $parts[2]

    $SafeName = $PkgName -replace "[/@:]", "__"
    $OutFile = Join-Path $OutputDir ("go_" + $SafeName + ".txt")

    Write-Host "Processing Go package: $PkgName"

    # Skip cs.opensource.google (no raw download available)
    if ($LicenseUrl -like "https://cs.opensource.google*") {
        Set-Content $OutFile @(
            "Package: $PkgName"
            "License: $LicenseType"
            "Source: $LicenseUrl"
            ""
            "This license is hosted on cs.opensource.google, which does not provide raw file downloads."
            "Please visit the URL manually to view the license."
        )
        continue
    }

    # Regex to match any version number like v2.11.0 or v3.0.0
    $pattern = 'https:\/\/github\.com\/wailsapp\/wails\/blob\/v\d+\.\d+\.\d+\/v\d+\/LICENSE'

    if ($LicenseUrl -match $pattern) {
        # Remove the "v2/" or "v3/" part after the version
        $LicenseUrl = $LicenseUrl -replace '/v\d+/', '/'
    }


    $Downloaded = $false

    # Prefer exact versioned GitHub license
    if ($LicenseUrl -like "https://github.com/*/blob/*") {
        $Downloaded = Get-GitHubVersionedLicense -Url $LicenseUrl -OutFile $OutFile
    }

    # Fallback: direct download
    if (-not $Downloaded) {
        try {
            Invoke-WebRequest -Uri $LicenseUrl -OutFile $OutFile -UseBasicParsing
            $Downloaded = $true
        }
        catch {
            Write-Warning "Failed to download license for $PkgName"
            continue
        }
    }

    $Header = "Package: $PkgName`nLicense: $LicenseType`nSource: $LicenseUrl`n`n"
    $Content = Get-Content $OutFile -Raw
    Set-Content -Path $OutFile -Value ($Header + $Content)
}

##############################################
# 2. Collect npm dependency licenses
##############################################

Write-Host "Collecting npm dependency licenses..."

Push-Location $FrontendDir
$NpmLicenses = npx license-checker-rseidelsohn --production --csv
Pop-Location

# Skip header
$NpmLicenses = $NpmLicenses | Select-Object -Skip 1

foreach ($line in $NpmLicenses) {

    # Parse CSV safely
    $parts = $line -split ",(?=(?:[^""]*""[^""]*"")*[^""]*$)"
    $Module = $parts[0].Trim('"')
    $License = $parts[1].Trim('"')
    $Repo = $parts[2].Trim('"')

    # Skip internal package
    if ($Module -eq "@eltheledge/audio-over-ip_frontend@0.0.0") {
        Write-Host "Skipping internal package: $Module"
        continue
    }

    $SafeName = $Module -replace "[/@:]", "__"
    $OutFile = Join-Path $OutputDir ("npm_" + $SafeName + ".txt")

    Write-Host "Processing npm module: $Module"

    $Downloaded = $false

    # If GitHub repo → download LICENSE from repo root (HEAD)
    if ($Repo -like "https://github.com/*") {
        $OwnerRepo = $Repo -replace "https://github.com/", ""
        $Owner, $RepoName = $OwnerRepo -split "/"

        $RawUrl = "https://raw.githubusercontent.com/$Owner/$RepoName/HEAD/LICENSE"

        try {
            Invoke-WebRequest -Uri $RawUrl -OutFile $OutFile -UseBasicParsing
            if ((Get-Content $OutFile -Raw).Length -gt 0) {
                $Downloaded = $true
            }
        }
        catch {
            # fallback below
        }
    }

    # Fallback: metadata only
    if (-not $Downloaded) {
        Set-Content $OutFile @(
            "Module: $Module"
            "License: $License"
            "Repository: $Repo"
            ""
            "License text could not be automatically retrieved."
        )
    }
    else {
        $Header = "Module: $Module`nLicense: $License`nRepository: $Repo`n`n"
        $Content = Get-Content $OutFile -Raw
        Set-Content -Path $OutFile -Value ($Header + $Content)
    }
}

##############################################
# 3. Create ZIP archive
##############################################

Write-Host "Creating release ZIP..."

Compress-Archive -Path @(
    $BinaryPath
    (Join-Path $ProjectRoot "LICENSE")
    $OutputDir
) -DestinationPath $ZipName -Force

##############################################
# 4. Print license summary
##############################################

Write-Host ""
Write-Host "========== LICENSE SUMMARY =========="

$LicenseCounts = @{}

Get-ChildItem $OutputDir -File | ForEach-Object {
    $content = Get-Content $_.FullName -Raw

    if ($content -match "License:\s*([A-Za-z0-9\.\-_]+)") {
        $lic = $Matches[1]
        if ($LicenseCounts.ContainsKey($lic)) {
            $LicenseCounts[$lic]++
        }
        else {
            $LicenseCounts[$lic] = 1
        }
    }
}

foreach ($key in $LicenseCounts.Keys) {
    Write-Host "$key : $($LicenseCounts[$key])"
}

Write-Host "====================================="
Write-Host "Done! Created: $ZipName"
