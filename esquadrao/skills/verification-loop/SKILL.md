---
name: verification-loop
description: Como verificar que uma mudança realmente funciona antes de considerá-la pronta — checklist de build, testes, lint, e verificação manual do fluxo afetado. Use antes de reportar qualquer tarefa como concluída.
---

# Loop de Verificação

Nenhuma tarefa está pronta só porque o código "parece certo". Antes de reportar conclusão:

1. **Build limpo**: rode o build/compile real (`go build ./...`, `tsc --noEmit`, `expo prebuild` conforme o app) — não confie em leitura visual do código.
2. **Testes**: rode a suíte relevante, não só o teste novo. Se a suíte inteira é lenta, rode ao menos o pacote/módulo afetado por completo.
3. **Lint/vet**: `go vet`, `golangci-lint`, `eslint`, `ruff` conforme a linguagem — pegam classes de erro que leitura visual não pega de forma confiável.
4. **Verificação manual do fluxo**: para mudança em UI ou fluxo de usuário, descreva o passo a passo que você seguiria para confirmar visualmente — mesmo que não consiga rodar o app, isso força pensar no caso real de uso.
5. **Efeito colateral**: essa mudança altera o comportamento de algo que não estava no pedido? Se sim, isso precisa estar explícito no reporte, não escondido.

Se qualquer um dos passos acima não foi possível de executar de fato (ex: sem acesso a dispositivo físico para testar app mobile), diga isso explicitamente em vez de reportar como verificado.
