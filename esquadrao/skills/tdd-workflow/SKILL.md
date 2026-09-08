---
name: tdd-workflow
description: Metodologia de TDD passo a passo — quando escrever o teste antes do código, como estruturar testes de tabela em Go, e como priorizar cobertura em apps com zero testes hoje. Use ao implementar qualquer lógica nova de negócio, especialmente envolvendo dinheiro ou máquina de estado.
---

# Fluxo de TDD

## O ciclo
1. **Vermelho**: escreva um teste que falha, descrevendo o comportamento em uma frase. Se não cabe em uma frase, o escopo do teste está grande demais — quebre em dois.
2. Rode o teste e confirme que falha pelo **motivo certo** (comportamento ausente, não erro de compilação/import).
3. **Verde**: escreva o mínimo de código para passar. Resista a generalizar antes de ter um segundo caso que peça generalização.
4. Rode a suíte inteira antes de seguir — um teste novo passando não vale nada se quebrou outro.
5. **Refatore** só com verde, nunca com vermelho.

## Testes de tabela (Go)
Para lógica com múltiplos casos de entrada/saída (ex: cálculo de taxa por zona com decaimento 3%→12%), use `t.Run` com uma slice de casos — inclua sempre os limites do intervalo (exatamente no ponto de transição), não só valores do meio.

## Priorizando em app com zero cobertura
Quando um app (ex: AppEntrega, AppRestaurante) tem zero ou quase zero testes, não tente cobrir tudo de uma vez. Ordem de prioridade:
1. Fluxo com maior risco financeiro (checkout, confirmação de pagamento).
2. Máquina de estado crítica (status do pedido: criado → aceito → em rota → entregue).
3. Autenticação/sessão.
4. Só depois, telas de exibição sem side-effect.

Registre a decisão de "não vamos testar X agora" explicitamente — dívida técnica anotada é diferente de dívida técnica esquecida.

## Testes de idempotência
Toda operação que credita/debita carteira ou processa webhook de pagamento precisa de pelo menos um teste que chama a operação duas vezes com a mesma chave de idempotência e confirma que o efeito não dobra.
