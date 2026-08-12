# ============================================================================
# Script de teste: Upload para Supabase Storage (PowerShell)
# ============================================================================
# Uso:
#   $env:SUPABASE_URL='https://seu-projeto.supabase.co'
#   $env:SUPABASE_SERVICE_ROLE_KEY='eyJhbG...'
#   .\scripts\test-supabase-upload.ps1
# ============================================================================

Write-Host "=== Teste de Upload para Supabase Storage ===" -ForegroundColor Yellow
Write-Host

# Verificar variaveis
if (-not $env:SUPABASE_URL) {
    Write-Host "ERRO: SUPABASE_URL nao configurado" -ForegroundColor Red
    exit 1
}

if (-not $env:SUPABASE_SERVICE_ROLE_KEY) {
    Write-Host "ERRO: SUPABASE_SERVICE_ROLE_KEY nao configurado" -ForegroundColor Red
    exit 1
}

Write-Host "✓ Variaveis configuradas" -ForegroundColor Green
Write-Host "  SUPABASE_URL: $($env:SUPABASE_URL.Substring(0, [Math]::Min(30, $env:SUPABASE_URL.Length)))..."
Write-Host

# Gerar imagem PNG placeholder (1x1 pixel vermelho)
Write-Host "Gerando imagem placeholder..." -ForegroundColor Yellow
$base64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwADhQGAWjR9awAAAABJRU5ErkJggg=="
$bytes = [Convert]::FromBase64String($base64)
$tempFile = "$env:TEMP\test-upload.png"
[System.IO.File]::WriteAllBytes($tempFile, $bytes)
Write-Host "✓ Imagem criada (1x1 pixel vermelho)" -ForegroundColor Green
Write-Host

# Preparar upload
$bucket = "fuudelivery-images"
$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
$filePath = "test/upload-test-$timestamp.png"
$fullUrl = "$env:SUPABASE_URL/storage/v1/object/$bucket/$filePath"

Write-Host "Fazendo upload: $filePath" -ForegroundColor Yellow

try {
    $headers = @{
        "Authorization" = "Bearer $env:SUPABASE_SERVICE_ROLE_KEY"
        "Content-Type" = "image/png"
    }
    
    $response = Invoke-WebRequest -Uri $fullUrl -Method Post -Headers $headers -InFile $tempFile -UseBasicParsing
    
    Write-Host
    Write-Host "✅ Upload bem-sucedido! (HTTP $($response.StatusCode))" -ForegroundColor Green
    Write-Host
    
    $publicUrl = "$env:SUPABASE_URL/storage/v1/object/public/$bucket/$filePath"
    Write-Host "URL publica:" -ForegroundColor Green
    Write-Host "  $publicUrl"
    Write-Host
    
    # Verificar URL publica
    try {
        $check = Invoke-WebRequest -Uri $publicUrl -Method Head -UseBasicParsing
        Write-Host "✅ URL publica acessivel!" -ForegroundColor Green
    } catch {
        Write-Host "⚠️  URL retornou erro - verifique policies RLS" -ForegroundColor Yellow
    }
} catch {
    Write-Host
    Write-Host "❌ Upload falhou" -ForegroundColor Red
    Write-Host $_.Exception.Message
    Write-Host
    Write-Host "Possiveis causas:" -ForegroundColor Yellow
    Write-Host "  1. Bucket 'fuudelivery-images' nao existe"
    Write-Host "  2. Chave API invalida"
    Write-Host "  3. Policies RLS bloqueando"
    exit 1
} finally {
    Remove-Item -Path $tempFile -Force -ErrorAction SilentlyContinue
}
