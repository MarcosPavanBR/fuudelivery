// Números do painel da loja (DashboardCharts). Funções puras, testadas em
// __tests__/dashboardStats.test.js.
import { itemName } from "./orderCard";

const DAY_NAMES = ["Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"];

// "AAAA-MM-DD" no fuso de quem está vendo. toISOString() é UTC: no Brasil,
// pedido das 21h contava como do dia seguinte.
export function localDayKey(date) {
  const d = new Date(date);
  if (Number.isNaN(d.getTime())) return "";
  const p = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
}

// Dia da semana de uma data "AAAA-MM-DD" do relatório. new Date("2026-09-19")
// é meia-noite UTC — no Brasil vira sexta 21h e o gráfico rotulava o dia
// anterior. Meio-dia local não troca de dia em fuso nenhum do país.
export function weekdayLabel(isoDay) {
  const d = new Date(`${String(isoDay).slice(0, 10)}T12:00:00`);
  return Number.isNaN(d.getTime()) ? String(isoDay) : DAY_NAMES[d.getDay()];
}

const NOT_SOLD = new Set(["DENIED", "CANCELLED"]);

export function countToday(orders, now = Date.now()) {
  const today = localDayKey(now);
  return (orders || []).filter((o) => localDayKey(o.created_at || o.CreatedAt) === today).length;
}

// Mais vendidos nos últimos `days` dias, por quantidade (recusados e
// cancelados não contam).
export function topProducts(orders, now = Date.now(), days = 7, limit = 5) {
  const since = now - days * 86400000;
  const counts = new Map();
  for (const o of orders || []) {
    const t = Date.parse(o.created_at || o.CreatedAt);
    if (Number.isNaN(t) || t < since || NOT_SOLD.has(o.status)) continue;
    for (const c of o.cart || []) {
      const name = itemName(c);
      counts.set(name, (counts.get(name) || 0) + (c.quantity || 0));
    }
  }
  return [...counts.entries()]
    .map(([name, count]) => ({ name, count }))
    .filter((p) => p.count > 0)
    .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name))
    .slice(0, limit);
}

export function formatBRL(value, compact = false) {
  return new Intl.NumberFormat("pt-BR", {
    style: "currency",
    currency: "BRL",
    maximumFractionDigits: compact ? 0 : 2,
    minimumFractionDigits: compact ? 0 : 2,
  }).format(value || 0);
}
