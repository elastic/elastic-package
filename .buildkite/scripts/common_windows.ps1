# Forcing to checkout again all the files with a correct autocrlf.
# Doing this here because we cannot set git clone options before.
function fixCRLF {
    Write-Host "-- Fixing CRLF in git checkout --"
    git config core.autocrlf input
    git rm --quiet --cached -r .
    git reset --quiet --hard
}

function ensureBinPath {
    $workDir = if ($env:WORKSPACE) { $env:WORKSPACE } else { $PWD.Path }
    $binDir = Join-Path $workDir "bin"
    if (-not (Test-Path $binDir)) { New-Item -ItemType Directory -Path $binDir | Out-Null }
    $env:PATH = "$binDir;$env:PATH"
    return $binDir
}

function withGolang($version) {
    Write-Host "--- Install golang (GVM)"
    $binDir = ensureBinPath
    $gvmExe = Join-Path $binDir "gvm-windows-amd64.exe"
    $gvmUrl = "https://github.com/andrewkroh/gvm/releases/download/$env:SETUP_GVM_VERSION/gvm-windows-amd64.exe"

    Write-Host "Installing GVM tool"
    $maxTries = 5
    for ($i = 1; $i -le $maxTries; $i++) {
        try {
            Invoke-WebRequest -Uri $gvmUrl -OutFile $gvmExe -UseBasicParsing
            break
        } catch {
            if ($i -eq $maxTries) { throw }
            Start-Sleep -Seconds 3
        }
    }

    Write-Host "Installing Go version $version"
    & $gvmExe --format=powershell $version | Invoke-Expression
    $env:PATH = "$(go env GOPATH)\bin;$env:PATH"
    go version
    go env
}
