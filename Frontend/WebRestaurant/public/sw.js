// FuuDelivery - WebRestaurant Service Worker
// Estratégia leve: shell offline (cache-first para assets estáticos),
// network-first para navegação, SEM cache para API/WebSocket.
const CACHE = "fuudelivery-web-v1";
const SHELL = ["/", "/index.html", "/manifest.json", "/favicon.svg"];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches.open(CACHE).then((cache) => cache.addAll(SHELL)).then(() => self.skipWaiting())
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener("fetch", (event) => {
  const { request } = event;
  if (request.method !== "GET") return;

  const url = new URL(request.url);

  // Nunca intercepta API nem WebSocket — dados de pedidos precisam ser frescos.
  if (url.pathname.startsWith("/ws") || url.pathname.startsWith("/payments")) return;
  if (url.origin !== self.location.origin && !url.hostname.includes("onrender.com")) {
    // CDN/fontes externas: deixa o navegador resolver.
    return;
  }
  if (url.origin !== self.location.origin) return; // API externa: network direto

  // Navegação: network-first com fallback para o shell (offline).
  if (request.mode === "navigate") {
    event.respondWith(
      fetch(request)
        .then((res) => {
          const copy = res.clone();
          caches.open(CACHE).then((cache) => cache.put("/index.html", copy));
          return res;
        })
        .catch(() => caches.match("/index.html"))
    );
    return;
  }

  // Assets estáticos: cache-first com atualização em background.
  event.respondWith(
    caches.match(request).then((cached) => {
      const network = fetch(request)
        .then((res) => {
          if (res.ok) {
            const copy = res.clone();
            caches.open(CACHE).then((cache) => cache.put(request, copy));
          }
          return res;
        })
        .catch(() => cached);
      return cached || network;
    })
  );
});
