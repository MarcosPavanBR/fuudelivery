---
name: credential-hygiene
description: Higiene de credenciais — o que nunca commitar, como rotacionar após vazamento, e checagem automática antes de push. Use PROACTIVELY antes de qualquer commit que toque configuração, documentação com exemplos, ou arquivos de ambiente — este projeto já teve vazamento recorrente do mesmo tipo em sessões diferentes.
---

# Higiene de Credenciais

## O padrão de erro já visto neste projeto (checar primeiro)
Vazamento de credencial de produção já ocorreu mais de uma vez aqui, migrando de arquivo em arquivo: primeiro em `CREDENTIALS.md`, depois reaparecendo em `.fuudelivery-config/DOCUMENTATION.md`. O padrão é sempre o mesmo — alguém documenta a configuração de produção "para referência" e inclui o valor real em vez de um placeholder. Antes de qualquer commit que toque um arquivo de documentação de configuração, confira se algum valor ali é um segredo real.

## Checklist antes de commit
- `grep` por padrões de chave/token comuns (`sk-`, `Bearer `, strings longas em Base64 perto de palavras como `key`, `secret`, `token`, `password`) nos arquivos alterados.
- Nenhum arquivo `.env` real no diff — só `.env.example` com placeholders.
- Backup de banco (`*.json`, `*.sql`, `db_backup_*`) nunca no diff — vai para storage com controle de acesso, nunca para o git.

## Se já vazou
1. Rotacione a credencial imediatamente — remover do histórico do git não invalida um segredo que já foi visto por qualquer clone do repositório.
2. Rotacione **todas** as credenciais relacionadas ao mesmo vazamento, não só a mais óbvia (ex: se o arquivo tinha Mongo + Render + gateway de pagamento, rotacione as três).
3. Adicione o arquivo/padrão ao `.gitignore` antes de recriar o arquivo de configuração, para não repetir o mesmo vazamento na próxima sessão.

## Prevenção estrutural
- Prefira um gerenciador de segredos (variáveis de ambiente da própria plataforma de deploy) a qualquer arquivo de documentação com valores reais — documentação deve descrever *quais* variáveis existem, nunca *quais são seus valores* em produção.
