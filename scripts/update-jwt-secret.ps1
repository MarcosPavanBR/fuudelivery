# Script para atualizar JWT_SECRET no Render
# Uso: .\scripts\update-jwt-secret.ps1

param(
    [string]$RenderApiKey = $env:RENDER_API_KEY
)

# Cores para output
function Write-Color {
    param([string]$Text, [string]$Color = "White")
    Write-Host $Text -ForegroundColor $Color
}

Write-Color "================================================================" "Yellow"
Write-Color "  Atualizar JWT_SECRET no Render - FuuDelivery" "Yellow"
Write-Color "================================================================" "Yellow"
Write-Host ""

# Verificar se RENDER_API_KEY está configurada
if (-not $RenderApiKey) {
    Write-Color "ERRO: RENDER_API_KEY não configurado" "Red"
    Write-Host ""
    Write-Host "Para obter a API key:"
    Write-Host "1. Acesse https://dashboard.render.com"
    Write-Host "2. Vá em Account Settings → API Keys"
    Write-Host "3. Crie uma nova API Key"
    Write-Host ""
    Write-Host "Para configurar:"
    Write-Host '  $env:RENDER_API_KEY="rnd_..."'
    exit 1
}

# Gerar novo JWT_SECRET
Write-Color "Gerando novo JWT_SECRET seguro..." "Green"
$bytes = New-Object Byte[] 32
[System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
$NEW_JWT_SECRET = ($bytes | ForEach-Object { $_.ToString("x2") }) -join ""
Write-Color "Novo JWT_SECRET: $NEW_JWT_SECRET" "Green"
Write-Host ""

# Service ID do Render (fuudelivery-api)
$SERVICE_ID = "srv-d9e55qf41pts73e8q8dg"

Write-Color "Atualizando JWT_SECRET no Render..." "Yellow"
Write-Host "Service ID: $SERVICE_ID"
Write-Host ""

# Atualizar via API
$body = @{
    envVars = @(
        @{
            key = "JWT_SECRET"
            value = $NEW_JWT_SECRET
        }
    )
} | ConvertTo-Json -Depth 3

try {
    $response = Invoke-RestMethod -Uri "https://api.render.com/v1/services/$SERVICE_ID/env-vars" `
        -Method Patch `
        -Headers @{
            "Authorization" = "Bearer $RenderApiKey"
            "Content-Type" = "application/json"
        } `
        -Body $body

    Write-Color "✅ JWT_SECRET atualizado com sucesso!" "Green"
    Write-Host ""
    Write-Color "Próximos passos:" "Yellow"
    Write-Host "1. O Render fará auto-deploy em ~2 minutos"
    Write-Host "2. Teste o sistema: https://fuudelivery-api-8y6l.onrender.com/health"
    Write-Host "3. Verifique se o login funciona: https://fuudelivery-admin-lv7f.onrender.com/"
}
catch {
    Write-Color "❌ Erro ao atualizar JWT_SECRET" "Red"
    Write-Host "Erro: $_"
    exit 1
}

Write-Host ""
Write-Color "IMPORTANTE:" "Yellow"
Write-Host "- Todas as sessões JWT anteriores serão invalidadas"
Write-Host "- Usuários precisarão fazer login novamente"
Write-Host "- O novo JWT_SECRET está salvo no Render"
Write-Host ""
Write-Color "Script concluído!" "Green"
