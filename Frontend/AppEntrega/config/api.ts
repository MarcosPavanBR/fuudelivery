// =============================================================
// CONFIG CENTRAL DA API — FONTE ÚNICA DE URLs DOS APPS MOBILE
// =============================================================
// Lista canônica de todas as URLs de produção: references/URLS.md
//
// Regra: builds de produção devem definir EXPO_PUBLIC_API_URL.
// Não há fallback hardcoded para produção.
// =============================================================

export const getApiUrl = (): string => {
  const url = process.env.EXPO_PUBLIC_API_URL
  if (!url) {
    throw new Error("EXPO_PUBLIC_API_URL não definida. Defina a variável antes do build.")
  }
  return url
}

export const getWsUrl = (): string =>
  process.env.EXPO_PUBLIC_WS_URL || getApiUrl().replace(/^http/, "ws")

export async function requestWsTicket(jwt: string): Promise<string> {
  const res = await fetch(`${getApiUrl()}/auth/ws-ticket`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${jwt}`,
      "Content-Type": "application/json",
    },
  })
  if (!res.ok) throw new Error(`WS ticket failed: ${res.status}`)
  const data = await res.json()
  return data.ticket
}

export default { getApiUrl, getWsUrl, requestWsTicket }
