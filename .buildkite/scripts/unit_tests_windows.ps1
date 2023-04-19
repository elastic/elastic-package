echo "--- Fixing CRLF in git checkout"
# Forcing to checkout again all the files with a correct autocrlf.
# Doing this here because we cannot set git clone options before.
git config core.autocrlf input
git rm --quiet --cached -r .
git reset --quiet --hard

echo "--- Installing golang"
choco install -y golang --version 1.20.3

echo "--- Updating session environment"
# refreshenv requires to have chocolatey profile installed
$env:ChocolateyInstall = Convert-Path "$((Get-Command choco).Path)\..\.."
Import-Module "$env:ChocolateyInstall\helpers\chocolateyProfile.psm1"

refreshenv

echo "--- Downloading Go modules"
go version
go mod download -x

echo "--- Running unit tests"
go version
go test ./...
