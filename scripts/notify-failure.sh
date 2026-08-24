#!/usr/bin/env bash
# notify-failure.sh — notifica falha de pipeline via Slack, Discord e/ou e-mail.
#
# Tudo opcional: só envia para os canais cujo secret estiver configurado.
# Nunca falha o pipeline (exit 0) — notificação é best-effort; se nenhum
# canal estiver configurado, loga um warning para o usuário configurar.
#
# Contexto (setado pelo workflow via env):
#   NOTIFY_TITLE   título da notificação (ex.: "Deploy FALHOU — fuudelivery")
#   NOTIFY_TEXT    corpo/motivo (ex.: resultados dos jobs)
#   NOTIFY_RUN_URL link do run no GitHub Actions
#   NOTIFY_SHA     SHA do commit
#   NOTIFY_REF     branch/ref
#
# Secrets:
#   SLACK_WEBHOOK_URL     -> Slack incoming webhook
#   DISCORD_WEBHOOK_URL   -> Discord webhook
#   SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, SMTP_TO, SMTP_FROM -> e-mail
set -uo pipefail

TITLE="${NOTIFY_TITLE:-Pipeline falhou}"
TEXT="${NOTIFY_TEXT:-}"
RUN_URL="${NOTIFY_RUN_URL:-}"
SHA="${NOTIFY_SHA:-}"
REF="${NOTIFY_REF:-}"

sent=0

# ── Slack ──────────────────────────────────────────────────────────────────
if [ -n "${SLACK_WEBHOOK_URL:-}" ]; then
  payload=$(python3 - "$TITLE" "$TEXT" "$RUN_URL" "$SHA" "$REF" <<'PY'
import json, sys
title, text, run_url, sha, ref = sys.argv[1:6]
blocks = [{"type": "section", "text": {"type": "mrkdwn", "text": f"*{title}*"}}]
if text:
    blocks.append({"type": "section", "text": {"type": "mrkdwn", "text": f"```{text}```"}})
fields = []
if run_url:
    fields.append({"type": "mrkdwn", "text": f"*Run:* <{run_url}|abrir>"})
if sha:
    fields.append({"type": "mrkdwn", "text": f"*Commit:* `{sha[:12]}`"})
if ref:
    fields.append({"type": "mrkdwn", "text": f"*Ref:* {ref}"})
if fields:
    blocks.append({"type": "section", "fields": fields})
print(json.dumps({"blocks": blocks}))
PY
  )
  code=$(curl -s -o /dev/null -w "%{http_code}" -m 20 -X POST \
    -H "Content-Type: application/json" -d "$payload" "$SLACK_WEBHOOK_URL" 2>/dev/null || echo "000")
  if [ "$code" = "200" ]; then
    echo "Slack: notificação enviada (HTTP $code)"
    sent=$((sent + 1))
  else
    echo "Slack: falhou ao enviar (HTTP $code)"
  fi
fi

# ── Discord ────────────────────────────────────────────────────────────────
if [ -n "${DISCORD_WEBHOOK_URL:-}" ]; then
  payload=$(python3 - "$TITLE" "$TEXT" "$RUN_URL" "$SHA" "$REF" <<'PY'
import json, sys
title, text, run_url, sha, ref = sys.argv[1:6]
fields = []
if run_url:
    fields.append({"name": "Run", "value": run_url, "inline": False})
if sha:
    fields.append({"name": "Commit", "value": f"`{sha[:12]}`", "inline": True})
if ref:
    fields.append({"name": "Ref", "value": ref, "inline": True})
embed = {"title": title, "description": text, "color": 15548997, "fields": fields}
print(json.dumps({"embeds": [embed]}))
PY
  )
  code=$(curl -s -o /dev/null -w "%{http_code}" -m 20 -X POST \
    -H "Content-Type: application/json" -d "$payload" "$DISCORD_WEBHOOK_URL" 2>/dev/null || echo "000")
  if [ "$code" = "204" ] || [ "$code" = "200" ]; then
    echo "Discord: notificação enviada (HTTP $code)"
    sent=$((sent + 1))
  else
    echo "Discord: falhou ao enviar (HTTP $code)"
  fi
fi

# ── E-mail (SMTP) ──────────────────────────────────────────────────────────
if [ -n "${SMTP_HOST:-}" ] && [ -n "${SMTP_TO:-}" ]; then
  if python3 - "$TITLE" "$TEXT" "$RUN_URL" "$SHA" "$REF" <<'PY'
import os, smtplib, sys
from email.mime.text import MIMEText

title, text, run_url, sha, ref = sys.argv[1:6]
body = f"{title}\n\n{text}\n" if text else f"{title}\n"
if run_url:
    body += f"\nRun: {run_url}\n"
if sha:
    body += f"Commit: {sha[:12]}\n"
if ref:
    body += f"Ref: {ref}\n"

msg = MIMEText(body)
msg["Subject"] = title
msg["From"] = os.environ.get("SMTP_FROM") or os.environ.get("SMTP_USER") or "noreply@fuudelivery"
msg["To"] = os.environ.get("SMTP_TO", "")

try:
    port = int(os.environ.get("SMTP_PORT", "587"))
    s = smtplib.SMTP(os.environ["SMTP_HOST"], port, timeout=20)
    try:
        s.starttls()
    except Exception:
        pass
    if os.environ.get("SMTP_USER"):
        s.login(os.environ["SMTP_USER"], os.environ.get("SMTP_PASS", ""))
    s.sendmail(msg["From"], [msg["To"]], msg.as_string())
    s.quit()
except Exception as exc:
    print(f"E-mail: erro ao enviar: {exc}", file=sys.stderr)
    sys.exit(1)
print("E-mail: notificação enviada")
PY
  then
    sent=$((sent + 1))
  else
    echo "E-mail: falhou ao enviar (veja o log acima)"
  fi
fi

# ── Resumo ─────────────────────────────────────────────────────────────────
if [ "$sent" -eq 0 ]; then
  echo "::warning::Nenhum canal de notificação configurado (SLACK_WEBHOOK_URL / DISCORD_WEBHOOK_URL / SMTP_*)."
fi
echo "notify-failure: $sent canal(ais) notificado(s)."
exit 0
