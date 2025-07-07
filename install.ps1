# LocalCloud Windows PowerShell Installer
# Usage: iwr -useb https://localcloud.sh/install.ps1 | iex

param(
    [string]$Version = "latest",
    [string]$InstallDir = "",
    [switch]$AddToPath = $true,
    [switch]$Force = $false
)

$ErrorActionPreference = 'Stop'

# Colors for output
function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    } else {
        $input | Write-Output
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

function Write-Success { Write-ColorOutput Green $args }
function Write-Info { Write-ColorOutput Cyan $args }
function Write-Warning { Write-ColorOutput Yellow $args }
function Write-Error { Write-ColorOutput Red $args }

# Banner
Write-Host ""
Write-Success "╔══════════════════════════════════════════════════════════════════╗"
Write-Success "║                          LocalCloud                              ║"
Write-Success "║              AI Development at Zero Cost                         ║"
Write-Success "║                                                                  ║"
Write-Success "║  Perfect for Claude Code, Cursor, Gemini CLI                    ║"
Write-Success "║  Programming Language Agnostic                                  ║"
Write-Success "║  Local-First Development                                         ║"
Write-Success "╚══════════════════════════════════════════════════════════════════╝"
Write-Host ""

# Check PowerShell version
if ($PSVersionTable.PSVersion.Major -lt 5) {
    Write-Error "PowerShell 5.0 or higher is required. You have version $($PSVersionTable.PSVersion)."
    exit 1
}

# Detect architecture
$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
Write-Info "Detected architecture: $arch"

# Determine install directory
if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "LocalCloud"
}

Write-Info "Install directory: $InstallDir"

# Check if already installed
$binaryPath = Join-Path $InstallDir "localcloud.exe"
if ((Test-Path $binaryPath) -and (-not $Force)) {
    Write-Warning "LocalCloud is already installed at $binaryPath"
    Write-Info "Use -Force to reinstall or run: localcloud --version"
    exit 0
}

try {
    # Get latest release info
    Write-Info "Fetching latest release information..."
    
    if ($Version -eq "latest") {
        $releaseUrl = "https://api.github.com/repos/localcloud-sh/localcloud/releases/latest"
        $release = Invoke-RestMethod -Uri $releaseUrl -UseBasicParsing
        $Version = $release.tag_name
    }
    
    Write-Info "Installing LocalCloud $Version"
    
    # Construct download URL
    $fileName = "localcloud-$Version-windows-$arch.zip"
    $downloadUrl = "https://github.com/localcloud-sh/localcloud/releases/download/$Version/$fileName"
    
    Write-Info "Downloading from: $downloadUrl"
    
    # Create temp directory
    $tempDir = Join-Path $env:TEMP "localcloud-install"
    $zipPath = Join-Path $tempDir $fileName
    
    if (Test-Path $tempDir) {
        Remove-Item $tempDir -Recurse -Force
    }
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    
    # Download the release
    Write-Info "Downloading LocalCloud..."
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath -UseBasicParsing
    
    # Extract the archive
    Write-Info "Extracting files..."
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::ExtractToDirectory($zipPath, $tempDir)
    
    # Find the binary
    $extractedBinary = Get-ChildItem -Path $tempDir -Name "localcloud-windows-$arch.exe" -Recurse | Select-Object -First 1
    if (-not $extractedBinary) {
        Write-Error "Could not find localcloud binary in downloaded archive"
        exit 1
    }
    
    $extractedPath = Join-Path $tempDir $extractedBinary
    
    # Create install directory
    Write-Info "Installing to $InstallDir..."
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    
    # Copy binary
    Copy-Item $extractedPath $binaryPath -Force
    
    # Add to PATH if requested
    if ($AddToPath) {
        Write-Info "Adding LocalCloud to PATH..."
        
        # Get current user PATH
        $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
        
        if ($userPath -notlike "*$InstallDir*") {
            $newPath = "$userPath;$InstallDir"
            [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
            
            # Update current session PATH
            $env:PATH = "$env:PATH;$InstallDir"
            
            Write-Success "[OK] Added $InstallDir to PATH"
        } else {
            Write-Info "LocalCloud directory already in PATH"
        }
    }
    
    # Clean up
    Remove-Item $tempDir -Recurse -Force
    
    # Verify installation
    Write-Info "Verifying installation..."
    $version = & $binaryPath --version 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Success "[OK] LocalCloud installed successfully!"
        Write-Success "[OK] Version: $version"
    } else {
        Write-Warning "Installation completed but version check failed"
    }
    
    Write-Host ""
    Write-Success "[SUCCESS] LocalCloud is ready to use!"
    Write-Host ""
    Write-Info "Quick Start (AI Assistant Mode):"
    Write-Host "  localcloud setup my-ai-app --preset=ai-dev --yes" -ForegroundColor White
    Write-Host "  cd my-ai-app" -ForegroundColor White
    Write-Host "  localcloud start" -ForegroundColor White
    Write-Host ""
    Write-Info "Interactive Setup:"
    Write-Host "  localcloud setup my-project" -ForegroundColor White
    Write-Host ""
    Write-Info "Get Help:"
    Write-Host "  localcloud --help" -ForegroundColor White
    Write-Host "  Documentation: https://docs.localcloud.sh" -ForegroundColor White
    Write-Host ""
    
    if ($AddToPath) {
        Write-Warning "Note: You may need to restart your terminal for PATH changes to take effect."
    }

} catch {
    Write-Error "Installation failed: $($_.Exception.Message)"
    Write-Error "Please report this issue at: https://github.com/localcloud-sh/localcloud/issues"
    exit 1
}