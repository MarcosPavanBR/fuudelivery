---
description: Extrai um padrão reutilizável de uma correção ou decisão recente e sugere adicioná-lo a uma skill existente
argument-hint: [descrição da correção/decisão a capturar]
---

Analise $ARGUMENTS e identifique se representa um padrão que vai se repetir (não um bug único isolado). Se sim, proponha a edição concreta (arquivo e trecho) na skill mais relevante em `skills/` para capturar esse aprendizado — não crie uma skill nova se uma existente já cobre o domínio.

Se for um erro genuinamente único sem chance de repetição, diga isso explicitamente em vez de forçar uma generalização.
