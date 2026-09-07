---
name: coding-standards
description: Padrões de código gerais (nomenclatura, tamanho de função, comentários, estrutura de pastas) para os projetos Go/TypeScript/Python deste time. Use sempre que estiver escrevendo código novo, não só ao revisar — consulte antes de decidir onde um arquivo novo deve morar ou como nomear algo.
---

# Padrões de Código

## Nomenclatura
- Go: `camelCase` para privado, `PascalCase` para exportado, nunca abreviações obscuras (`dlvBoy` não, `deliveryPartner` sim).
- TypeScript: `camelCase` para variáveis/funções, `PascalCase` para componentes e tipos, arquivos de componente React em `PascalCase.tsx`.
- Nomes de handler HTTP descrevem a ação e o recurso: `HandleGetOrderStatus`, não `HandleOrder2`.

## Tamanho e estrutura
- Função com mais de ~40 linhas é candidata a quebrar — não é regra rígida, mas se você não consegue descrever o que ela faz em uma frase, provavelmente faz duas coisas.
- Um arquivo por responsabilidade. `zone.go` cuida de zona; lógica de decaimento de taxa vive em `split_decay_job.go`, não misturada em `zone.go` só porque usa zona como input.
- Pastas espelham domínio, não camada técnica: prefira `internal/wallet/{service.go,repository.go}` a `internal/services/wallet.go` + `internal/repositories/wallet.go` separados por tipo técnico.

## Comentários
- Comente o *porquê*, nunca o *o quê* (o código já diz o quê). `// idempotência: mesma chave de evento pode chegar 2x via retry do gateway` é útil; `// soma o valor` não é.
- Todo `TODO` tem um nome ou issue associado — `TODO(marcos): revisar após migração de schema`, nunca `TODO` solto sem dono.

## Estrutura de commit
- Um commit, uma mudança lógica. Não misture refactor com feature nova no mesmo commit.
- Mensagem no imperativo: "corrige idempotência do webhook", não "corrigido" ou "correção de".

## Quando isso não se aplica
Scripts de análise única/descartáveis (investigação pontual) não precisam seguir todo o rigor acima — mas nunca commitados como se fossem código de produção.
