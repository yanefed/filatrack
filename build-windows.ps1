<#
.SYNOPSIS
Builds a portable 64-bit Windows bundle of FilaTrack with GTK 4/libadwaita.

.EXAMPLE
.\build-windows.ps1 -InstallDependencies -Archive

.EXAMPLE
.\build-windows.ps1 -Console
#>
[CmdletBinding()]
param(
    [string]$MSYS2Root = $(if ($env:MSYS2_ROOT) { $env:MSYS2_ROOT } else { "C:\msys64" }),
    [string]$OutputDir = "dist\FilaTrack-windows-amd64",
    [switch]$InstallDependencies,
    [switch]$IncludeLocales,
    [switch]$Console,
    [switch]$Archive
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$ProjectRoot = $PSScriptRoot
$MSYS2Root = [IO.Path]::GetFullPath($MSYS2Root)
$UcrtRoot = Join-Path $MSYS2Root "ucrt64"
$UcrtBin = Join-Path $UcrtRoot "bin"
$Bash = Join-Path $MSYS2Root "usr\bin\bash.exe"
$Pacman = Join-Path $MSYS2Root "usr\bin\pacman.exe"
$Go = Join-Path $UcrtBin "go.exe"
$Gcc = Join-Path $UcrtBin "gcc.exe"
$PkgConfig = Join-Path $UcrtBin "pkg-config.exe"
$ObjDump = Join-Path $UcrtBin "objdump.exe"
$PixbufQuery = Join-Path $UcrtBin "gdk-pixbuf-query-loaders.exe"

if (-not (Test-Path $Pacman)) {
    throw "MSYS2 was not found in '$MSYS2Root'. Install it from https://www.msys2.org/ or pass -MSYS2Root."
}

if ($InstallDependencies) {
    $Packages = @(
        "mingw-w64-ucrt-x86_64-go",
        "mingw-w64-ucrt-x86_64-gcc",
        "mingw-w64-ucrt-x86_64-binutils",
        "mingw-w64-ucrt-x86_64-pkgconf",
        "mingw-w64-ucrt-x86_64-gobject-introspection",
        "mingw-w64-ucrt-x86_64-gtk4",
        "mingw-w64-ucrt-x86_64-libadwaita"
    )
    Write-Host "Installing MSYS2 UCRT64 dependencies..."
    & $Bash -lc ("pacman -S --needed --noconfirm " + ($Packages -join " "))
    if ($LASTEXITCODE -ne 0) {
        throw "pacman failed with exit code $LASTEXITCODE"
    }
}

$RequiredTools = @($Go, $Gcc, $PkgConfig, $ObjDump, $PixbufQuery)
foreach ($Tool in $RequiredTools) {
    if (-not (Test-Path $Tool)) {
        throw "Required tool '$Tool' is missing. Run this script once with -InstallDependencies."
    }
}

$OutputPath = if ([IO.Path]::IsPathRooted($OutputDir)) {
    [IO.Path]::GetFullPath($OutputDir)
} else {
    [IO.Path]::GetFullPath((Join-Path $ProjectRoot $OutputDir))
}
$DistRoot = [IO.Path]::GetFullPath((Join-Path $ProjectRoot "dist"))
if (-not $OutputPath.StartsWith($DistRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
    throw "OutputDir must be inside '$DistRoot'."
}

if (Test-Path $OutputPath) {
    Remove-Item -LiteralPath $OutputPath -Recurse -Force
}
New-Item -ItemType Directory -Path $OutputPath | Out-Null

$OldPath = $env:PATH
$OldCGO = $env:CGO_ENABLED
$OldGOOS = $env:GOOS
$OldGOARCH = $env:GOARCH
$OldCC = $env:CC
$OldPKGConfigPath = $env:PKG_CONFIG_PATH

try {
    $env:PATH = "$UcrtBin;$($env:PATH)"
    $env:CGO_ENABLED = "1"
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $env:CC = $Gcc
    $env:PKG_CONFIG_PATH = (Join-Path $UcrtRoot "lib\pkgconfig") + ";" + (Join-Path $UcrtRoot "share\pkgconfig")

    $ExePath = Join-Path $OutputPath "FilaTrack.exe"
    $LdFlags = if ($Console) { "-s -w" } else { "-s -w -H=windowsgui" }
    Write-Host "Building FilaTrack.exe..."
    Push-Location $ProjectRoot
    try {
        & $Go build -trimpath -ldflags $LdFlags -o $ExePath ./cmd/filatrack
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed with exit code $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }

    # Recursively collect only non-system DLLs referenced by the executable and
    # its dependencies. This keeps the bundle much smaller than copying ucrt64/bin.
    $Visited = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
    $Queue = [Collections.Generic.Queue[string]]::new()
    $Queue.Enqueue($ExePath)
    $PixbufQueryTarget = Join-Path $OutputPath "gdk-pixbuf-query-loaders.exe"
    Copy-Item -LiteralPath $PixbufQuery -Destination $PixbufQueryTarget
    $Queue.Enqueue($PixbufQuery)
    foreach ($HelperName in @("gspawn-win64-helper.exe", "gspawn-win64-helper-console.exe")) {
        $HelperSource = Join-Path $UcrtBin $HelperName
        if (Test-Path $HelperSource) {
            Copy-Item -LiteralPath $HelperSource -Destination (Join-Path $OutputPath $HelperName)
            $Queue.Enqueue($HelperSource)
        }
    }
    while ($Queue.Count -gt 0) {
        $Binary = $Queue.Dequeue()
        if (-not $Visited.Add([IO.Path]::GetFullPath($Binary))) {
            continue
        }
        $Dump = & $ObjDump -p $Binary 2>$null
        foreach ($Line in $Dump) {
            if ($Line -notmatch '^\s*DLL Name:\s*(.+?)\s*$') {
                continue
            }
            $DllName = $Matches[1]
            $DllSource = Join-Path $UcrtBin $DllName
            if (-not (Test-Path $DllSource)) {
                continue # Windows system DLL.
            }
            $DllTarget = Join-Path $OutputPath $DllName
            if (-not (Test-Path $DllTarget)) {
                Copy-Item -LiteralPath $DllSource -Destination $DllTarget
            }
            $Queue.Enqueue($DllSource)
        }
    }

    function Copy-RuntimeTree([string]$RelativePath, [bool]$Required = $false) {
        $Source = Join-Path $UcrtRoot $RelativePath
        if (-not (Test-Path $Source)) {
            if ($Required) { throw "Required GTK runtime directory '$Source' is missing." }
            return
        }
        $Target = Join-Path $OutputPath $RelativePath
        $TargetParent = Split-Path -Parent $Target
        New-Item -ItemType Directory -Path $TargetParent -Force | Out-Null
        Copy-Item -LiteralPath $Source -Destination $Target -Recurse -Force

        # Modules are loaded dynamically and do not appear in FilaTrack.exe's import table.
        Get-ChildItem -LiteralPath $Target -Filter "*.dll" -Recurse | ForEach-Object {
            $Queue.Enqueue($_.FullName)
        }
    }

    Copy-RuntimeTree "lib\gdk-pixbuf-2.0"
    Copy-RuntimeTree "lib\gtk-4.0"
    Copy-RuntimeTree "lib\gio\modules"
    Copy-RuntimeTree "share\glib-2.0\schemas" $true
    Copy-RuntimeTree "share\icons\Adwaita" $true
    Copy-RuntimeTree "share\icons\hicolor"
    Copy-RuntimeTree "etc\fonts"
    Copy-RuntimeTree "share\fontconfig"
    Copy-RuntimeTree "share\mime"
    Copy-RuntimeTree "etc\gtk-4.0"
    if ($IncludeLocales) {
        Copy-RuntimeTree "share\locale"
    }

    # Resolve dependencies of dynamically loaded GTK and pixbuf modules too.
    while ($Queue.Count -gt 0) {
        $Binary = $Queue.Dequeue()
        if (-not $Visited.Add([IO.Path]::GetFullPath($Binary))) {
            continue
        }
        $Dump = & $ObjDump -p $Binary 2>$null
        foreach ($Line in $Dump) {
            if ($Line -match '^\s*DLL Name:\s*(.+?)\s*$') {
                $DllName = $Matches[1]
                $DllSource = Join-Path $UcrtBin $DllName
                if (Test-Path $DllSource) {
                    $DllTarget = Join-Path $OutputPath $DllName
                    if (-not (Test-Path $DllTarget)) {
                        Copy-Item -LiteralPath $DllSource -Destination $DllTarget
                    }
                    $Queue.Enqueue($DllSource)
                }
            }
        }
    }

    # The pixbuf loader cache contains absolute paths. Generate it on the target
    # machine so the bundle remains relocatable after extracting the archive.
    $LauncherPath = Join-Path $OutputPath "Run-FilaTrack.cmd"
    $Launcher = @'
@echo off
setlocal EnableExtensions EnableDelayedExpansion
set "APPDIR=%~dp0"
set "XDG_DATA_DIRS=!APPDIR!share;!XDG_DATA_DIRS!"
set "GSETTINGS_SCHEMA_DIR=!APPDIR!share\glib-2.0\schemas"
set "FONTCONFIG_PATH=!APPDIR!etc\fonts"

for /d %%D in ("!APPDIR!lib\gdk-pixbuf-2.0\*") do (
    if exist "%%~fD\loaders" set "GDK_PIXBUF_MODULEDIR=%%~fD\loaders"
)

if defined GDK_PIXBUF_MODULEDIR (
    if defined LOCALAPPDATA (set "FILATRACK_CACHE=!LOCALAPPDATA!\FilaTrack") else (set "FILATRACK_CACHE=!TEMP!\FilaTrack")
    if not exist "!FILATRACK_CACHE!" mkdir "!FILATRACK_CACHE!" >nul 2>&1
    set "GDK_PIXBUF_MODULE_FILE=!FILATRACK_CACHE!\loaders.cache"
    "!APPDIR!gdk-pixbuf-query-loaders.exe" --update-cache >nul 2>&1
)

"!APPDIR!FilaTrack.exe" %*
exit /b !ERRORLEVEL!
'@
    [IO.File]::WriteAllText($LauncherPath, $Launcher, [Text.UTF8Encoding]::new($false))

    if ($Archive) {
        $ArchivePath = Join-Path $DistRoot "FilaTrack-windows-amd64.zip"
        if (Test-Path $ArchivePath) {
            Remove-Item -LiteralPath $ArchivePath -Force
        }
        Compress-Archive -Path (Join-Path $OutputPath "*") -DestinationPath $ArchivePath -CompressionLevel Optimal
        Write-Host "Archive created: $ArchivePath"
    }

    Write-Host "Windows bundle created: $OutputPath"
} finally {
    $env:PATH = $OldPath
    $env:CGO_ENABLED = $OldCGO
    $env:GOOS = $OldGOOS
    $env:GOARCH = $OldGOARCH
    $env:CC = $OldCC
    $env:PKG_CONFIG_PATH = $OldPKGConfigPath
}
