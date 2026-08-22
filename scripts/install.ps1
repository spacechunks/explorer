& {
    Set-StrictMode -Version Latest
    $ErrorActionPreference = "Stop"

    $repositoryUrl = "https://github.com/spacechunks/explorer"
    $binaryName = "explorerctl.exe"
    $tempDirectory = $null

    function Write-Brand {
        Write-Host ""
        if ($env:NO_COLOR) {
            Write-Host "✦" -NoNewline
        } else {
            Write-Host "✦" -ForegroundColor Magenta -NoNewline
        }
        Write-Host " Chunk Explorer"
        Write-Host ""
    }

    function Write-Step([string] $Message) {
        Write-Host "  " -NoNewline
        if ($env:NO_COLOR) {
            Write-Host "✓" -NoNewline
        } else {
            Write-Host "✓" -ForegroundColor Green -NoNewline
        }
        Write-Host " $Message"
    }

    function Get-LatestVersion {
        $response = Invoke-WebRequest -Uri "$repositoryUrl/releases/latest" -Method Head -UseBasicParsing
        if ($response.BaseResponse.PSObject.Properties.Name -contains "ResponseUri") {
            $releaseUri = $response.BaseResponse.ResponseUri
        } elseif ($response.BaseResponse.PSObject.Properties.Name -contains "RequestMessage") {
            $releaseUri = $response.BaseResponse.RequestMessage.RequestUri
        } else {
            throw "GitHub did not return the latest release URL."
        }

        return [Uri]::UnescapeDataString($releaseUri.Segments[-1].TrimEnd("/"))
    }

    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor `
            [Net.SecurityProtocolType]::Tls12

        $architectureName = if ($env:PROCESSOR_ARCHITEW6432) {
            $env:PROCESSOR_ARCHITEW6432
        } else {
            $env:PROCESSOR_ARCHITECTURE
        }

        $architecture = switch ($architectureName.ToUpperInvariant()) {
            "AMD64" { "amd64" }
            "ARM64" { "arm64" }
            default { throw "Unsupported architecture: $architectureName" }
        }

        $version = $env:EXPLORER_VERSION
        if ([string]::IsNullOrWhiteSpace($version)) {
            $version = Get-LatestVersion
        }
        if (-not $version.StartsWith("v")) {
            $version = "v$version"
        }
        if ($version -notmatch '^v[A-Za-z0-9._+\-]+$') {
            throw "Invalid Explorer CLI version: $version"
        }

        $installDirectory = $env:EXPLORER_INSTALL_DIR
        if ([string]::IsNullOrWhiteSpace($installDirectory)) {
            $installDirectory = Join-Path $env:LOCALAPPDATA "Programs\Explorer"
        }
        $installDirectory = $installDirectory.TrimEnd("\", "/")
        if ([string]::IsNullOrWhiteSpace($installDirectory)) {
            throw "The installation directory cannot be empty."
        }

        $archiveName = "explorer_${version}_windows_${architecture}.tar.gz"
        $checksumName = "explorer_${version}_sha256sums"
        $releaseBinary = "explorer_${version}_windows_${architecture}.exe"
        $escapedVersion = [Uri]::EscapeDataString($version)
        $releaseUrl = "$repositoryUrl/releases/download/$escapedVersion"

        $tempDirectory = Join-Path ([IO.Path]::GetTempPath()) "explorer-install-$([Guid]::NewGuid())"
        New-Item -ItemType Directory -Path $tempDirectory | Out-Null
        $archivePath = Join-Path $tempDirectory $archiveName
        $checksumPath = Join-Path $tempDirectory $checksumName

        Write-Brand
        Write-Host "  Installing $version for Windows $architecture"
        Write-Host ""

        Invoke-WebRequest -Uri "$releaseUrl/$archiveName" -OutFile $archivePath -UseBasicParsing
        Invoke-WebRequest -Uri "$releaseUrl/$checksumName" -OutFile $checksumPath -UseBasicParsing
        Write-Step "Downloaded $archiveName"

        $escapedArchiveName = [Regex]::Escape($archiveName)
        $checksumLine = Get-Content -LiteralPath $checksumPath |
            Where-Object { $_ -match "^([a-fA-F0-9]{64})\s+$escapedArchiveName$" } |
            Select-Object -First 1
        if ($null -eq $checksumLine) {
            throw "The checksum manifest does not contain $archiveName."
        }

        $expectedChecksum = ($checksumLine -split '\s+')[0]
        $actualChecksum = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash
        if ($actualChecksum -ine $expectedChecksum) {
            throw "Checksum verification failed for $archiveName."
        }
        Write-Step "Verified the SHA-256 checksum"

        $tar = Get-Command "tar.exe" -ErrorAction SilentlyContinue
        if ($null -eq $tar) {
            throw "tar.exe is required to extract the Explorer CLI release."
        }
        & $tar.Source -xzf $archivePath -C $tempDirectory
        if ($LASTEXITCODE -ne 0) {
            throw "Could not extract $archiveName."
        }

        $sourceBinary = Join-Path $tempDirectory $releaseBinary
        if (-not (Test-Path -LiteralPath $sourceBinary -PathType Leaf)) {
            throw "The release archive does not contain $releaseBinary."
        }

        New-Item -ItemType Directory -Path $installDirectory -Force | Out-Null
        $installedBinary = Join-Path $installDirectory $binaryName
        Copy-Item -LiteralPath $sourceBinary -Destination $installedBinary -Force
        Write-Step "Installed explorerctl to $installedBinary"

        $pathEntries = @($env:Path -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
        if (-not ($pathEntries | Where-Object { $_.TrimEnd("\") -ieq $installDirectory.TrimEnd("\") })) {
            $env:Path = "$installDirectory;$env:Path"

            $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
            $userPathEntries = @($userPath -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
            if (-not ($userPathEntries | Where-Object { $_.TrimEnd("\") -ieq $installDirectory.TrimEnd("\") })) {
                $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) {
                    $installDirectory
                } else {
                    "$installDirectory;$userPath"
                }
                [Environment]::SetEnvironmentVariable("Path", $newUserPath, "User")
            }
        }

        Write-Host ""
        Write-Host "Explorer CLI is ready."
        Write-Host "Run " -NoNewline
        Write-Host "explorerctl --help" -NoNewline
        Write-Host " to get started."
        Write-Host ""
    } catch {
        if ($env:NO_COLOR) {
            Write-Host "`n  Error: $($_.Exception.Message)`n" -ErrorAction Continue
        } else {
            Write-Host "`n  Error: " -ForegroundColor Red -NoNewline
            Write-Host "$($_.Exception.Message)`n"
        }
        throw
    } finally {
        if ($null -ne $tempDirectory -and (Test-Path -LiteralPath $tempDirectory)) {
            Remove-Item -LiteralPath $tempDirectory -Recurse -Force
        }
    }
}
