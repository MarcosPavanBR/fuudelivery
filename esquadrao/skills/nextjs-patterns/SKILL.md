---
name: nextjs-patterns
description: Padrões de Next.js para portais de conteúdo (ex: portal de imigração, sites de renda passiva) — SSG/ISR para SEO, estrutura de conteúdo, e painel administrativo. Use ao construir ou evoluir qualquer site Next.js orientado a SEO/conteúdo.
---

# Padrões Next.js (portais de conteúdo)

## Renderização
- Páginas de conteúdo evergreen (guias, artigos) usam SSG com ISR (`revalidate`) — nunca client-side rendering puro para conteúdo que precisa ser indexado.
- Metadata (`generateMetadata`) por página, nunca um título/descrição genérico repetido em todo o site — isso mata SEO de cauda longa.

## Estrutura de conteúdo
- Organize por jornada do usuário, não por tipo técnico de arquivo — ex: `/planejamento`, `/pouso`, `/legalizacao` para um portal de imigração, cada um agrupando os artigos daquela etapa.
- URLs estáveis e amigáveis (`/guia/visto-d7`, não `/post?id=123`) — mudar URL depois de indexado custa ranking.

## Painel administrativo
- CRUD de conteúdo separado do site público, atrás de autenticação própria — nunca reaproveite a mesma rota pública com um `?admin=true`.
- Editor salva rascunho e publicado separadamente, permitindo revisar antes de publicar.

## Conformidade (ver skill lgpd-compliance)
- Banner de consentimento de cookies antes de carregar qualquer script de analytics/anúncio, com opção real de recusar (não só "ok" decorativo).

## Performance
- Imagens via `next/image` com dimensões explícitas — evita layout shift que penaliza Core Web Vitals e, por consequência, ranking.
