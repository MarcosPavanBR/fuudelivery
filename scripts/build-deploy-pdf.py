#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
build-deploy-pdf.py — Gera scripts/deploy-vps.pdf a partir de scripts/deploy-vps.md
====================================================================================
Como usar (após atualizar o Markdown, ex.: nova versão de Go/Node/Redis):

    python scripts/build-deploy-pdf.py

O script:
  1. Converte o Markdown (subconjunto usado no guia) para HTML estilizado;
  2. Imprime o HTML em PDF via Microsoft Edge ou Google Chrome headless;
  3. Salva em scripts/deploy-vps.pdf (o HTML temporário é removido).

Pré-requisitos: Python 3 + Microsoft Edge ou Google Chrome instalados (Windows/Mac/Linux).
Sem dependências externas (stdlib apenas).
"""

import os
import re
import subprocess
import sys
import tempfile

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
MD_PATH = os.path.join(ROOT, "scripts", "deploy-vps.md")
PDF_PATH = os.path.join(ROOT, "scripts", "deploy-vps.pdf")

CSS = """
:root { color-scheme: light; }
* { box-sizing: border-box; }
body {
  font-family: "Segoe UI", "Helvetica Neue", Arial, sans-serif;
  font-size: 10.5pt; line-height: 1.55; color: #1c2733; margin: 0;
  padding: 28px 36px;
}
h1 {
  font-size: 21pt; color: #0f766e; border-bottom: 3px solid #0f766e;
  padding-bottom: 10px; margin: 0 0 14px; line-height: 1.25;
}
h2 {
  font-size: 15pt; color: #0f766e; border-bottom: 1.5px solid #99d6cf;
  padding-bottom: 5px; margin: 26px 0 10px; page-break-after: avoid;
}
h3 { font-size: 12.5pt; color: #134e4a; margin: 18px 0 8px; page-break-after: avoid; }
h4 { font-size: 11pt; color: #134e4a; margin: 14px 0 6px; page-break-after: avoid; }
p { margin: 7px 0; }
strong { color: #0b3d39; }
code {
  font-family: "Cascadia Code", Consolas, "Courier New", monospace;
  font-size: 0.88em; background: #f1f5f4; color: #0f5132;
  padding: 1px 5px; border-radius: 4px;
}
pre {
  background: #0f172a; color: #e2e8f0; padding: 12px 14px; border-radius: 8px;
  overflow-x: auto; font-size: 9pt; line-height: 1.45; margin: 10px 0;
  page-break-inside: avoid;
}
pre code { background: none; color: inherit; padding: 0; font-size: inherit; }
table {
  border-collapse: collapse; width: 100%; margin: 10px 0; font-size: 9.5pt;
  page-break-inside: auto;
}
th { background: #0f766e; color: #fff; text-align: left; }
th, td { border: 1px solid #cbd5d1; padding: 6px 9px; vertical-align: top; }
tr:nth-child(even) td { background: #f7faf9; }
blockquote {
  border-left: 4px solid #0f766e; background: #f0fdfa; margin: 10px 0;
  padding: 8px 14px; color: #155e55; border-radius: 0 6px 6px 0;
}
blockquote code { background: #d7f0ec; }
ul, ol { margin: 7px 0 7px 4px; padding-left: 22px; }
li { margin: 3px 0; }
hr { border: none; border-top: 1.5px solid #cbd5d1; margin: 20px 0; }
a { color: #0f766e; text-decoration: none; }
@media print {
  @page { size: A4; margin: 14mm; }
  body { padding: 0; }
  h2, h3 { break-after: avoid; }
  pre, table, blockquote { break-inside: avoid; }
}
"""


# ─────────────────────────────────────────────────────────────────────────
# Conversor de Markdown (subconjunto usado no guia)
# ─────────────────────────────────────────────────────────────────────────

def inline(text: str) -> str:
    """Aplica formatação inline: `code`, **bold**, [link](url).

    Escapa primeiro `<`, `>` e `&` para que texto literal como `<IP_DO_VPS>`
    não seja interpretado como tag HTML (sumiria na renderização).
    """
    text = text.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
    text = re.sub(r"`([^`]+)`", r"<code>\1</code>", text)
    text = re.sub(r"\*\*([^*]+)\*\*", r"<strong>\1</strong>", text)
    text = re.sub(
        r"\[([^\]]+)\]\(([^)]+)\)",
        lambda m: '<a href="%s">%s</a>' % (m.group(2), m.group(1)),
        text,
    )
    return text


def convert(md: str) -> str:
    lines = md.replace("\r\n", "\n").split("\n")
    out: list[str] = []
    i, n = 0, len(lines)

    while i < n:
        line = lines[i]

        # Código em bloco
        if line.strip().startswith("```"):
            lang = line.strip()[3:].strip()
            buf: list[str] = []
            i += 1
            while i < n and not lines[i].strip().startswith("```"):
                buf.append(lines[i])
                i += 1
            i += 1  # pula o fechamento
            html = "\n".join(buf)
            # escape básico
            html = html.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
            lang_attr = f' class="language-{lang}"' if lang else ""
            out.append(f"<pre><code{lang_attr}>{html}</code></pre>")
            continue

        # Cabeçalhos
        m = re.match(r"^(#{1,4})\s+(.*)$", line)
        if m:
            level = len(m.group(1))
            out.append(f"<h{level}>{inline(m.group(2))}</h{level}>")
            i += 1
            continue

        # Linha horizontal
        if re.match(r"^\s*---+\s*$", line):
            out.append("<hr/>")
            i += 1
            continue

        # Citação
        if line.startswith(">"):
            buf = []
            while i < n and lines[i].startswith(">"):
                buf.append(lines[i][1:].strip())
                i += 1
            # inline() por linha (escapa <>) e só depois junta com <br/> real,
            # senão o escape converteria o próprio <br/> em texto literal
            out.append("<blockquote>" + "<br/>".join(inline(x) for x in buf) + "</blockquote>")
            continue

        # Tabela
        if "|" in line and i + 1 < n and re.match(r"^\s*\|?[\s:|-]+\|?\s*$", lines[i + 1]):
            header = [c.strip() for c in line.strip().strip("|").split("|")]
            i += 2  # pula cabeçalho + separador
            rows = []
            while i < n and "|" in lines[i] and lines[i].strip():
                cells = [c.strip() for c in lines[i].strip().strip("|").split("|")]
                rows.append("<tr>" + "".join(f"<td>{inline(c)}</td>" for c in cells) + "</tr>")
                i += 1
            thead = "<tr>" + "".join(f"<th>{inline(h)}</th>" for h in header) + "</tr>"
            out.append(f"<table><thead>{thead}</thead><tbody>{''.join(rows)}</tbody></table>")
            continue

        # Lista
        m = re.match(r"^(\s*)([-*]|\d+\.)\s+(.*)$", line)
        if m:
            ordered = m.group(2).rstrip(".").isdigit()
            tag = "ol" if ordered else "ul"
            buf = []
            while i < n:
                mm = re.match(r"^(\s*)([-*]|\d+\.)\s+(.*)$", lines[i])
                if not mm:
                    break
                buf.append(f"<li>{inline(mm.group(3))}</li>")
                i += 1
            out.append(f"<{tag}>{''.join(buf)}</{tag}>")
            continue

        # Parágrafo
        if line.strip():
            out.append(f"<p>{inline(line.strip())}</p>")
        i += 1

    return "\n".join(out)


# ─────────────────────────────────────────────────────────────────────────
# Localização do navegador (Edge > Chrome)
# ─────────────────────────────────────────────────────────────────────────

CANDIDATES = {
    "win32": [
        r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
        r"C:\Program Files\Microsoft\Edge\Application\msedge.exe",
        r"C:\Program Files\Google\Chrome\Application\chrome.exe",
        r"C:\Program Files (x86)\Google\Chrome\Application\chrome.exe",
    ],
    "darwin": [
        "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
        "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    ],
    "linux": [
        "/usr/bin/microsoft-edge",
        "/usr/bin/microsoft-edge-stable",
        "/usr/bin/google-chrome",
        "/usr/bin/chromium-browser",
        "/usr/bin/chromium",
    ],
}


def find_browser() -> str:
    for path in CANDIDATES.get(sys.platform, []):
        if os.path.isfile(path):
            return path
    # Tenta no PATH
    for name in ("msedge", "microsoft-edge", "google-chrome", "chromium"):
        which = subprocess.run(["which", name], capture_output=True, text=True)
        if which.returncode == 0 and which.stdout.strip():
            return which.stdout.strip()
    raise SystemExit(
        "ERRO: Microsoft Edge ou Google Chrome não encontrado.\n"
        "Instale um deles ou ajuste CANDIDATES em build-deploy-pdf.py."
    )


def main() -> int:
    if not os.path.isfile(MD_PATH):
        raise SystemExit(f"ERRO: {MD_PATH} não existe.")

    html = (
        "<!DOCTYPE html><html lang='pt-BR'><head><meta charset='utf-8'>"
        f"<title>FuuDelivery — Deploy em VPS</title>"
        f"<style>{CSS}</style></head><body>"
        f"{convert(open(MD_PATH, encoding='utf-8').read())}"
        "</body></html>"
    )

    # O HTML temporário fica AO LADO do PDF (scripts/): o Edge/Chrome headless
    # deste ambiente falha em imprimir arquivos fora do diretório do projeto
    # (arquivos em %%TEMP%% retornam rc=0 sem gerar PDF).
    html_path = os.path.join(ROOT, "scripts", "._deploy-vps_tmp.html")
    with open(html_path, "w", encoding="utf-8") as f:
        f.write(html)

    browsers = [find_browser()]
    # Fallback: se o primeiro navegador falhar, tenta os demais encontrados
    import shutil as _shutil
    for cand in CANDIDATES.get(sys.platform, []):
        if os.path.isfile(cand) and cand not in browsers:
            browsers.append(cand)

    last_err = ""
    for browser in browsers:
        if os.path.exists(PDF_PATH):
            os.unlink(PDF_PATH)
        # Perfil isolado: sem ele, o navegador já aberto (instância normal)
        # recebe o comando e NÃO imprime o PDF (rc=0 sem gerar arquivo).
        profile = tempfile.mkdtemp(prefix="deploy-vps-prof-")
        cmd = [
            browser,
            "--headless",
            "--disable-gpu",
            "--no-pdf-header-footer",
            "--virtual-time-budget=3000",
            "--user-data-dir=" + profile,
            "--print-to-pdf=" + PDF_PATH,
            html_path,
        ]
        print(f"Gerando PDF com: {browser}")
        try:
            result = subprocess.run(cmd, capture_output=True, text=True, timeout=120)
        finally:
            _shutil.rmtree(profile, ignore_errors=True)
        if os.path.isfile(PDF_PATH) and os.path.getsize(PDF_PATH) > 1000:
            break
        last_err = (result.stderr or result.stdout or "")[-300:]
    else:
        os.unlink(html_path)
        raise SystemExit(
            "ERRO: nenhum navegador gerou o PDF.\n" + last_err
        )
    os.unlink(html_path)

    if result.returncode != 0 or not os.path.isfile(PDF_PATH):
        print(result.stderr[-2000:])
        raise SystemExit("ERRO: falha ao gerar o PDF.")

    size = os.path.getsize(PDF_PATH)
    pages = 0
    try:
        with open(PDF_PATH, "rb") as f:
            pages = len(re.findall(rb"/Type\s*/Page[^s]", f.read()))
    except Exception:
        pass

    print(f"OK: {PDF_PATH}")
    print(f"    {size / 1024:.0f} KB, ~{pages} páginas")
    return 0


if __name__ == "__main__":
    sys.exit(main())
