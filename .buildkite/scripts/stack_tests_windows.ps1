$ErrorActionPreference = "Stop" # set -e

# Forcing to checkout again all the files with a correct autocrlf.
# Doing this here because we cannot set git clone options before.
function fixCRLF {
    Write-Host "-- Fixing CRLF in git checkout --"
    git config core.autocrlf input
    git rm --quiet --cached -r .
    git reset --quiet --hard
}

function withGolang($version) {
    # Avoid conflicts with previous installations.
    Remove-Item env:GOROOT

    Write-Host "-- Install golang $version --"
    choco install -y golang --version $version
    setupChocolateyPath
    go version
    go env
}

function getEngine() {
    docker info --format '{{.OSType}}'
}

function withDockerDesktop($version) {
    Write-Host "-- Install docker desktop $version --"
    choco install -y docker-desktop --version $version
    setupChocolateyPath

    # Ensure that docker is running with the linux engine.
    Write-Host "-- Enable Linux docker engine"
    Start-Process -FilePath 'C:\Program Files\Docker\Docker\DockerCli.exe' -ArgumentList '-SwitchLinuxEngine'
    Restart-Service -Name Docker

    $count = 0
    while ($true) {
      #Check that the platform engine has started
      docker info 1>$null 2>$null

      if ($LASTEXITCODE -eq 0) {
        #Check that the engine has switched
        $engine = getEngine

        if ($LASTEXITCODE -eq 0 -and $engine -eq "linux") {
            #Success
            break
        }
      }

      $count += 1
      if ($count -ge 60) {
        Write-Error "Timed out waiting for engine to start"
      }

      Start-Sleep 1
    }

    Write-Host "--- Docker Info"
    docker info
}

function setupChocolateyPath() {
    $env:ChocolateyInstall = Convert-Path "$((Get-Command choco).Path)\..\.."
    Import-Module "$env:ChocolateyInstall\helpers\chocolateyProfile.psm1"
    refreshenv
}


fixCRLF

withGolang $env:GO_VERSION
withDockerDesktop "4.30.0"

echo "--- Downloading Go modules"
go version
go mod download -x

echo "--- Running stack tests"
go version
$ErrorActionPreference = "Continue" # set +e
go run . stack up -v -d
$EXITCODE=$LASTEXITCODE
$ErrorActionPreference = "Stop"

Exit $EXITCODE
