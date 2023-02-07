

echo "--- Installing make"
choco install -y make

echo "--- Installing golang"
choco install -y golang --version 1.19.5

echo "--- Updating session environment"
# refreshenv requires to have chocolatey profile installed
$env:ChocolateyInstall = Convert-Path "$((Get-Command choco).Path)\..\.."
Import-Module "$env:ChocolateyInstall\helpers\chocolateyProfile.psm1"

refreshenv

echo "--- Running unit tests"
go version
make test-go-ci-win

exit 1
