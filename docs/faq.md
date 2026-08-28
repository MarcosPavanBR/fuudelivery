# Perguntas Frequentes (FAQ) - FuuDelivery

## Geral

### O que é o FuuDelivery?
O FuuDelivery é uma plataforma de delivery colaborativa (cooperativa) que conecta restaurantes, clientes e entregadores com taxas significativamente menores que o modelo tradicional (5-12% vs 27-33% do iFood).

### Como funciona o modelo cooperativo?
Diferente de plataformas tradicionais onde a empresa lucra com taxas altas, o FuuDelivery distribui os lucros entre os participantes (restaurante, entregador e plataforma). A taxa é apenas para cobrir custos operacionais.

### Quais são as taxas?
- **Restaurante**: 5-12% (vs 27-33% do iFood)
- **Entregador**: Recebe 100% da taxa de entrega
- **Plataforma**: Recebe uma porcentagem mínima para manutenção

## Para Restaurantes

### Como me cadastrar?
1. Acesse o painel do restaurante (WebRestaurant ou AppRestaurante)
2. Clique em "Cadastre-se"
3. Preencha os dados do restaurante
4. Aguarde aprovação do admin

### Como gerenciar meu cardápio?
1. Faça login no painel
2. Acesse "Cardápio"
3. Adicione/edite/remove produtos
4. Adicione fotos (upload para Supabase)

### Como recebo meus pagamentos?
- Pagamentos são processados via PIX (AbacatePay)
- Split automático: valor é dividido entre restaurante e plataforma
- Saque disponível na carteira digital

### Como aceito um pedido?
1. Notificação sonora de novo pedido
2. Acesse "Pedidos" → "Em Análise"
3. Clique em "Aceitar" ou "Recusar"
4. Prepare o pedido
5. Marque como "Pronto para Entrega"

## Para Clientes

### Como faço um pedido?
1. Abra o AppComida
2. Selecione um restaurante da sua zona
3. Escolha os itens do cardápio
4. Finalize o checkout
5. Pague via PIX (QR Code)
6. Acompanhe a entrega em tempo real

### Como acompanho minha entrega?
- Tela de tracking com mapa em tempo real
- Notificações push a cada etapa (preparo, coleta, entrega)
- Chat com o entregador (em breve)

### Como solicito reembolso?
1. Acesse "Histórico de Pedidos"
2. Selecione o pedido
3. Clique em "Solicitar Reembolso"
4. Explique o motivo
5. Aguarde análise (até 48h)

### Onde vejo minha carteira digital?
1. Acesse "Perfil" → "Minha Carteira"
2. Veja saldo disponível
3. Solicite saque via PIX

## Para Entregadores

### Como me cadio?
1. Abra o AppEntrega
2. Clique em "Cadastre-se"
3. Preencha dados pessoais
4. Envie documentos (CNH, veículo)
5. Aguarde verificação

### Como recebo entregas disponíveis?
- Notificação de entrega disponível na sua zona
- Acesse "Entregas Disponíveis"
- Aceite a entrega (quem chegar primeiro leva)

### Como confirmo uma entrega?
1. Navegue até o restaurante
2. Confirme a coleta (botão + geolocalização)
3. Navegue até o cliente
4. Confirme a entrega
5. Valor é creditado na sua carteira

### Como funciona o matching de entregas?
- Sistema de calibration baseado em zona
- Matching por proximidade e disponibilidade
- Reclaim loop: entregas não aceitas são redistribuídas

## Técnico

### Quais são as tecnologias usadas?
- **Backend**: Go (Golang)
- **Banco de dados**: PostgreSQL (Supabase)
- **Cache/Filas**: Redis (Redis Streams)
- **Frontend Web**: React + Vite
- **Apps Mobile**: React Native + Expo
- **Deploy**: Render.com
- **Pagamentos**: AbacatePay (PIX)

### Como rodo o projeto localmente?
Veja `docs/guia-deploy.md` para instruções detalhadas.

### Onde reporto bugs?
- GitHub Issues: https://github.com/MarcosPavanBR/fuudelivery/issues
- Use o template de bug report

### Como contribuo?
Veja `docs/contribuicao.md` para o guia completo.

## Segurança

### Meus dados estão seguros?
Sim! O FuuDelivery usa:
- JWT com expiração para autenticação
- bcrypt para senhas (nunca em texto plano)
- HTTPS em todas as comunicações
- Rate limiting para prevenir ataques
- RBAC para controle de acesso

### Como reporto uma vulnerabilidade?
- Envie email para: (se disponível)
- Ou abra uma issue com label `security`
- Nunca publique vulnerabilidades publicamente

### Vocês seguem LGPD?
Sim. Dados pessoais são tratados conforme a LGPD:
- Apenas dados necessários são coletados
- Usuário pode solicitar exclusão
- Dados não são compartilhados com terceiros sem consentimento

## Pagamentos

### Quais formas de pagamento são aceitas?
- **PIX**: Forma principal (via AbacatePay)
- **Cartão**: Em breve (via Asaas)

### Como funciona o split de pagamento?
1. Cliente paga o total via PIX
2. Webhook confirma pagamento
3. Sistema calcula split (restaurante + plataforma)
4. Valores são creditados automaticamente
5. Restaurante e entregador podem sacar via PIX

### O que é chargeback?
É quando o cliente contesta um pagamento junto ao banco. O FuuDelivery:
1. Registra o chargeback
2. Bloqueia valores envolvidos
3. Analisa evidências
4. Resolve em até 30 dias
5. Debita do restaurante se procedente

### Como funciona a carteira digital?
- Saldo disponível para saque
- Histórico de transações
- Saque mínimo: R$ 10,00
- Saque via PIX: instantâneo

## Suporte

### Não consigo fazer login
1. Verifique email e senha
2. Clique em "Esqueci minha senha"
3. Verifique se a conta foi aprovada
4. Entre em contato: (se disponível)

### App trava ou buga
1. Feche e abra novamente
2. Verifique sua conexão
3. Atualize o app
4. Reporte o bug com prints

### Não recebo notificações
1. Verifique se notificações estão habilitadas
2. Verifique conexão com internet
3. Reinicie o app
4. Verifique configurações do celular

---

**Última atualização**: 11/08/2026
