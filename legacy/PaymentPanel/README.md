# Legacy — PaymentPanel (arquivado em 2026-08-23)

Este painel standalone de pagamentos foi **arquivado** por decisão do projeto:

- Todas as rotas de pagamento vivem no monolito (`fuudelivery-api`) desde a
  remoção do serviço `fuudelivery-payment` (2026-08).
- O **WebAdmin já possui uma aba Financeiro completa**, que cobre as mesmas
  funcionalidades com identidade visual atualizada.
- Não é deployado: o `render.yaml` não o referencia e o CI Gate não o builda.

## Se precisar restaurar

```bash
mv legacy/PaymentPanel Frontend/PaymentPanel
```

E re-adicionar o serviço no `render.yaml`. Antes de qualquer mudança aqui,
lembre que `index.html` apontava para serviços mortos — revise as URLs.
