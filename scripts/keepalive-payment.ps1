# Keep Alive — FuuDelivery Payment Service
# Pinga o health check a cada 10 minutos para evitar sono no Render free tier
# Executar: powershell -File scripts/keepalive-payment.ps1
param(
    [int]$IntervalMinutes = 10,
    [int]$MaxRuns = -1
)
$url = "https://fuudelivery-payment.onrender.com/health"
$count = 0
while ($MaxRuns -eq -1 -or $count -lt $MaxRuns) {
    $count++
    try {
        $r = Invoke-WebRequest -Uri $url -TimeoutSec 60 -UseBasicParsing
        Write-Host "[$(Get-Date -Format 'HH:mm:ss')] PING $count : $($r.StatusCode) $($r.Content)"
    } catch {
        Write-Host "[$(Get-Date -Format 'HH:mm:ss')] PING $count : OFFLINE — tentando despertar..."
    }
    Start-Sleep -Seconds ($IntervalMinutes * 60)
}