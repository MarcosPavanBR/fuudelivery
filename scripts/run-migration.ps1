# Script para executar a migration no banco de dados do Render
# Uso: .\scripts\run-migration.ps1

param(
    [string]$ConnectionString = $env:DB_CONNECTION_STRING
)

function Write-Color {
    param([string]$Text, [string]$Color = "White")
    Write-Host $Text -ForegroundColor $Color
}

Write-Color "================================================" "Yellow"
Write-Color "  FuuDelivery - Executar Migration Corretiva" "Yellow"
Write-Color "================================================" "Yellow"
Write-Host ""

# Verificar se DB_CONNECTION_STRING está configurada
if (-not $ConnectionString) {
    Write-Color "ERRO: DB_CONNECTION_STRING não configurado" "Red"
    Write-Host ""
    Write-Host "Para obter a connection string:"
    Write-Host "1. Acesse https://dashboard.render.com"
    Write-Host "2. Vá em fuudelivery-api → Environment"
    Write-Host "3. Copie o valor de DB_CONNECTION_STRING"
    Write-Host ""
    Write-Host "Para configurar:"
    Write-Host '  $env:DB_CONNECTION_STRING="postgresql://..."'
    exit 1
}

Write-Color "DB_CONNECTION_STRING configurado" "Green"
Write-Host ""

# Verificar se psql está disponível
if (-not (Get-Command psql -ErrorAction SilentlyContinue)) {
    Write-Color "ERRO: psql não encontrado" "Red"
    Write-Host "Instale o PostgreSQL client:"
    Write-Host "  Windows: choco install postgresql"
    Write-Host "  Ou baixe de: https://www.postgresql.org/download/windows/"
    exit 1
}

Write-Color "Executando migration..." "Yellow"
Write-Host ""

# Executar migration
& psql $ConnectionString -f "scripts\migrate-fix-all.sql"

if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Color "✅ Migration executada com sucesso!" "Green"
    Write-Host ""
    Write-Color "Próximos passos:" "Yellow"
    Write-Host "1. Reinicie o serviço no Render (ou espere auto-deploy)"
    Write-Host "2. Verifique os logs: deve mostrar [CALIBRATION] Starting calibration cycle"
    Write-Host "3. Teste a API: curl https://fuudelivery-api-8y6l.onrender.com/health"
} else {
    Write-Color "❌ Erro ao executar migration" "Red"
    exit 1
}
