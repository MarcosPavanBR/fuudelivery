---
name: deployment-patterns
description: Padrões de deploy — Docker, CI/CD, health checks, rollback, e checklist pré-deploy para plataformas como Render. Use antes de qualquer deploy em produção ou ao configurar pipeline de CI/CD.
---

# Padrões de Deploy

## Antes de todo deploy em produção
1. Suíte de testes passou no CI, não só localmente.
2. `security-reviewer` (agente) rodou nas mudanças desde o último deploy — não só na mudança individual mais recente.
3. Variáveis de ambiente de produção conferidas contra um `.env.example` atualizado — nenhuma nova variável obrigatória sem default documentada.
4. Migração de banco, se houver, testada em ambiente de staging antes, com plano de rollback claro.

## Docker
- Multi-stage build: estágio de build separado do estágio final de execução, imagem final sem ferramentas de build/compiladores.
- Nunca copie `.env` para dentro da imagem — variáveis de ambiente entram em runtime via configuração da plataforma.

## Health checks
- Endpoint de health check real (confere conexão com banco, não só "processo está de pé") usado pela plataforma de deploy para decidir se a instância está pronta para receber tráfego.

## Rollback
- Deploy sempre reversível para a versão anterior em um comando — se a versão anterior depender de uma migração que a nova reverteu, isso precisa estar documentado antes do deploy, não descoberto durante um incidente.

## CI/CD
- Pipeline roda: lint → testes → build → (se tudo passar) deploy. Nunca pule etapas para "economizar tempo" em deploy de sexta-feira.
