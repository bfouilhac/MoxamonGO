param(
    [Parameter(Mandatory=$true)]
    [Alias("a")]
    [string]$Address,
    
    [string]$Version = "1.0.0",
    [string]$User = "moxa"
)

$ErrorActionPreference = "Stop"

Write-Host "=== MOXA Config Server - Build & Deploy ===" -ForegroundColor Cyan
Write-Host "Cible: $User@$Address" -ForegroundColor Yellow
Write-Host ""

# Étape 1: Compilation
Write-Host "[1/3] Compilation pour ARM..." -ForegroundColor Green
$env:GOOS = "linux"
$env:GOARCH = "arm"
$env:GOARM = "7"
$env:CGO_ENABLED = "0"

go build -ldflags="-s -w" -o moxa-config ./cmd/server
if ($LASTEXITCODE -ne 0) {
    Write-Host "Erreur de compilation" -ForegroundColor Red
    exit 1
}
Write-Host "Compilation OK" -ForegroundColor Green

# Étape 2: Package
Write-Host "[2/3] Creation du package..." -ForegroundColor Green
$packageDir = "moxa-config-$Version"
$packageFile = "$packageDir.tar.gz"

Remove-Item -Recurse -Force $packageDir -ErrorAction SilentlyContinue
Remove-Item -Force $packageFile -ErrorAction SilentlyContinue

New-Item -ItemType Directory -Path $packageDir | Out-Null
Copy-Item moxa-config "$packageDir/"
Copy-Item scripts/install.sh "$packageDir/"

tar czf $packageFile $packageDir
if ($LASTEXITCODE -ne 0) {
    Write-Host "Erreur creation archive" -ForegroundColor Red
    exit 1
}
Write-Host "Package: $packageFile" -ForegroundColor Green

# Étape 3: Transfert
Write-Host "[3/3] Transfert vers $Address..." -ForegroundColor Green
scp $packageFile "${User}@${Address}:/tmp/"
if ($LASTEXITCODE -ne 0) {
    Write-Host "Erreur transfert SCP" -ForegroundColor Red
    exit 1
}
Write-Host "Transfert OK" -ForegroundColor Green

# Nettoyage local
Remove-Item -Recurse -Force $packageDir -ErrorAction SilentlyContinue
Remove-Item -Force moxa-config -ErrorAction SilentlyContinue
Remove-Item -Force $packageFile -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "=== Transfert termine ===" -ForegroundColor Cyan
Write-Host ""
Write-Host "Pour installer, connecte-toi puis execute :" -ForegroundColor Yellow
Write-Host "  ssh $User@$Address" -ForegroundColor White
Write-Host "  cd /tmp && tar xzf $packageFile && cd $packageDir && sudo sh ./install.sh" -ForegroundColor White
Write-Host ""
Write-Host "Interface : http://${Address}:8080" -ForegroundColor Yellow