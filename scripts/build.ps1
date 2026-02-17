param(
    [Parameter(Mandatory=$true)]
    [Alias("a")]
    [string]$Address,

    [string]$Version = "1.0.0",
    [string]$User = "moxa"
)

$ErrorActionPreference = "Stop"

Write-Host "=== MoxamonGO Server - Build & Deploy ===" -ForegroundColor Cyan
Write-Host "Cible: $User@$Address" -ForegroundColor Yellow
Write-Host ""

# Étape 1: Compilation serveur (ARM)
Write-Host "[1/4] Compilation serveur pour ARM..." -ForegroundColor Green
$env:GOOS = "linux"
$env:GOARCH = "arm"
$env:GOARM = "7"
$env:CGO_ENABLED = "0"

go build -ldflags="-s -w" -o moxa-config ./cmd/server
if ($LASTEXITCODE -ne 0) {
    Write-Host "Erreur de compilation serveur" -ForegroundColor Red
    exit 1
}
Write-Host "Serveur OK" -ForegroundColor Green

# Étape 2: Compilation scanner (Windows)
Write-Host "[2/4] Compilation scanner Windows..." -ForegroundColor Green
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"

go build -ldflags="-s -w" -o moxa-scanner.exe ./cmd/scanner
if ($LASTEXITCODE -ne 0) {
    Write-Host "Erreur compilation scanner" -ForegroundColor Red
    exit 1
}
Write-Host "Scanner OK: moxa-scanner.exe" -ForegroundColor Green

# Étape 3: Package
Write-Host "[3/4] Creation du package..." -ForegroundColor Green
$packageDir = "moxa-config-$Version"
$packageFile = "$packageDir.tar.gz"

Remove-Item -Recurse -Force $packageDir -ErrorAction SilentlyContinue
Remove-Item -Force $packageFile -ErrorAction SilentlyContinue

New-Item -ItemType Directory -Path $packageDir | Out-Null
Copy-Item moxa-config "$packageDir/"
Copy-Item scripts/install.sh "$packageDir/"

# Convertir les fins de ligne en LF (Unix)
$content = Get-Content "$packageDir/install.sh" -Raw
$content = $content -replace "`r`n", "`n"
[System.IO.File]::WriteAllText("$packageDir/install.sh", $content, [System.Text.Encoding]::UTF8)

tar czf $packageFile $packageDir
if ($LASTEXITCODE -ne 0) {
    Write-Host "Erreur creation archive" -ForegroundColor Red
    exit 1
}
Write-Host "Package: $packageFile" -ForegroundColor Green

# Étape 4: Transfert
Write-Host "[4/4] Transfert vers $Address..." -ForegroundColor Green
scp $packageFile "${User}@${Address}:/tmp/"
if ($LASTEXITCODE -ne 0) {
    Write-Host "Erreur transfert SCP" -ForegroundColor Red
    exit 1
}
Write-Host "Transfert OK" -ForegroundColor Green

# Nettoyage local (garder le scanner)
Remove-Item -Recurse -Force $packageDir -ErrorAction SilentlyContinue
Remove-Item -Force moxa-config -ErrorAction SilentlyContinue
Remove-Item -Force $packageFile -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "=== Build termine ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "Scanner disponible: .\moxa-scanner.exe" -ForegroundColor Yellow
Write-Host ""
Write-Host "Pour installer sur le MOXA:" -ForegroundColor Yellow
Write-Host "  ssh $User@$Address" -ForegroundColor White
Write-Host "  cd /tmp && tar xzf moxa-config-$Version.tar.gz && cd moxa-config-$Version && sudo sh ./install.sh" -ForegroundColor White
Write-Host ""
Write-Host "Interface: https://${Address}:8080" -ForegroundColor Yellow