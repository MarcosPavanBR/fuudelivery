# ============================================================================
# Script: Listar env vars do Render (PowerShell)
# ============================================================================
# Uso:
#   $env:RENDER_API_KEY="rnd_..."
#   .\scripts\list-render-env-vars.ps1
# ============================================================================

Write-Host "====================================================================" -ForegroundColor Cyan
Write-Host "  Auditoria de Env Vars do Render - FuuDelivery" -ForegroundColor Cyan
Write-Host "====================================================================" -ForegroundColor Cyan
Write-Host

# Verificar API key
if (-not $env:RENDER_API_KEY) {
    Write-Host "ERRO: RENDER_API_KEY não configurado" -ForegroundColor Red
    Write-Host
    Write-Host "Como obter:"
    Write-Host "  1. Acesse https://dashboard.render.com"
    Write-Host "  2. Account Settings → API Keys"
    Write-Host "  3. Create API Key"
    Write-Host
    Write-Host "Depois execute:"
    Write-Host '  $env:RENDER_API_KEY="rnd_..."'
    Write-Host '  .\scripts\list-render-env-vars.ps1'
    exit 1
}

# Serviços
$services = @{
    "fuudelivery-api" = "srv-d9e55qf41pts73e8q8dg"
    "fuudelivery-payment" = "srv-d9gego3rjlhs739jgrfg"
    "fuudelivery-web" = "srv-d9edpar7uimc73fdotp0"
    "fuudelivery-admin" = "srv-d9elp2n41pts73f5kvf0"
    "fuudelivery-payment-panel" = "srv-d9gefarrjlhs739jdl90"
}

# Env vars sensíveis
$sensitiveVars = @(
    "JWT_SECRET", "ADMIN_PASSWORD", "MONGO_URI", "DB_CONNECTION_STRING",
    "REDIS_URL", "ABACATE_PAY_API_KEY", "ABACATE_PAY_WEBHOOK_SECRET",
    "SUPABASE_URL", "SUPABASE_SERVICE_ROLE_KEY"
)

foreach ($serviceName in $services.Keys) {
    $serviceId = $services[$serviceName]
    
    Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
    Write-Host "📦 $serviceName" -ForegroundColor Green -NoNewline
    Write-Host " ($serviceId)"
    Write-Host "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━" -ForegroundColor Cyan
    
    try {
        $headers = @{ "Authorization" = "Bearer $env:RENDER_API_KEY" }
        $response = Invoke-RestMethod -Uri "https://api.render.com/v1/services/$serviceId/env-vars" -Headers $headers
        
        Write-Host "  Total de env vars: $($response.Count)"
        Write-Host
        Write-Host "  🔒 Env vars SENSÍVEIS:" -ForegroundColor Red
        
        $foundSensitive = 0
        foreach ($var in $sensitiveVars) {
            $envVar = $response | Where-Object { $_.key -eq $var }
            if ($envVar) {
                $value = $envVar.value
                if ($value.Length -gt 10) {
                    $masked = "$($value.Substring(0, 6))...$($value.Substring($value.Length - 4))"
                } else {
                    $masked = "***"
                }
                Write-Host "    • $var = $masked"
                $foundSensitive++
            }
        }
        
        if ($foundSensitive -eq 0) {
            Write-Host "    (nenhum encontrado)"
        }
    } catch {
        Write-Host "  ❌ Erro: $($_.Exception.Message)" -ForegroundColor Red
    }
    
    Write-Host
}

Write-Host "====================================================================" -ForegroundColor Cyan
Write-Host "📋 ENV VARS QUE PRECISAM SER ROTACIONADAS:" -ForegroundColor Yellow
Write-Host "====================================================================" -ForegroundColor Cyan
foreach ($var in $sensitiveVars) {
    Write-Host "  • $var"
}
