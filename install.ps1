# Installs the latest agy-switch release.
#   irm https://raw.githubusercontent.com/renatormc/agy-switcher/master/install.ps1 | iex
# Environment: AGY_SWITCH_INSTALL_DIR (default %LOCALAPPDATA%\Programs\agy-switch),
#              AGY_SWITCH_VERSION (e.g. v0.3.0; default latest).
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

$repo = 'renatormc/agy-switcher'
$installDir = if ($env:AGY_SWITCH_INSTALL_DIR) { $env:AGY_SWITCH_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\agy-switch' }
$version = if ($env:AGY_SWITCH_VERSION) { $env:AGY_SWITCH_VERSION } else { 'latest' }

$cpu = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
switch ($cpu) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' { $arch = 'arm64' }
    default { throw "agy-switch: unsupported architecture $cpu" }
}

$asset = "agy-switch_windows_$arch.exe"
if ($version -eq 'latest') {
    $url = "https://github.com/$repo/releases/latest/download/$asset"
} else {
    $url = "https://github.com/$repo/releases/download/$version/$asset"
}

New-Item -ItemType Directory -Force -Path $installDir | Out-Null
$exe = Join-Path $installDir 'agy-switch.exe'
$tmp = "$exe.download"
Write-Host "Downloading $url"
Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing
Move-Item -Force $tmp $exe

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$parts = @($userPath -split ';' | Where-Object { $_ })
if ($parts -notcontains $installDir) {
    [Environment]::SetEnvironmentVariable('Path', (($parts + $installDir) -join ';'), 'User')
    Write-Host "Added $installDir to your user PATH. Open a new terminal to use it."
}
if (($env:Path -split ';') -notcontains $installDir) { $env:Path = "$env:Path;$installDir" }

Write-Host "Installed $(& $exe --version) to $exe"
