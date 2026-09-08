---
name: lgpd-compliance
description: Conformidade com a LGPD (Lei Geral de Proteção de Dados, Brasil) — base legal, consentimento, retenção e direitos do titular. Use PROACTIVELY sempre que o projeto coletar, armazenar ou processar dado pessoal (CPF, localização, foto, histórico de pedido) — equivalente brasileiro ao GDPR europeu.
---

# Conformidade LGPD

## Base legal
Todo dado pessoal coletado precisa de uma base legal explícita (consentimento, execução de contrato, obrigação legal) — "guardamos porque pode ser útil depois" não é base legal válida.

## Consentimento
- Banner de cookies/analytics com opção real de recusar, não só um botão "ok" decorativo que na prática já carregou os scripts antes do clique.
- Consentimento para uso de localização (ex: AppEntrega, AppComida) é específico para essa finalidade — não reaproveite um consentimento genérico de "termos de uso" para cobrir coleta de localização em tempo real.

## Retenção e minimização
- Dado pessoal tem prazo de retenção definido, não "para sempre por via das dúvidas" — histórico de localização de entrega, por exemplo, não precisa ser mantido indefinidamente após a entrega concluída.
- Colete só o dado necessário para a finalidade declarada — CPF só onde há exigência real (nota fiscal, pagamento), não em todo cadastro.

## Direitos do titular
- Mecanismo real (não só uma cláusula no termo de uso) para o usuário solicitar exclusão ou exportação dos próprios dados.

## Incidente
- Vazamento de dado pessoal (ex: backup de banco versionado com hash de senha) é, sob a LGPD, um incidente de segurança com possível obrigação de comunicação — trate a descoberta de um vazamento como prioridade máxima, não como item de backlog.
