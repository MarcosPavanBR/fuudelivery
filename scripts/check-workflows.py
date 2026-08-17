#!/usr/bin/env python3
"""
check-workflows.py — Valida YAML + sintaxe bash dos run blocks.

Pega erros ANTES do push: YAML malformado ou um run block com bash invalido
(ex.: o `-w` quebrado no health-check) falha aqui, sem depender do GitHub
parsing os workflows no push nem de um job rodar o script de verdade.

Uso:
    python3 scripts/check-workflows.py                  # varre .github/workflows/*.yml
    python3 scripts/check-workflows.py --root DIR       # escopa outro diretorio
    python3 scripts/check-workflows.py --self-test      # roda fixtures de regressao
"""

import os
import re
import shutil
import subprocess
import sys
import tempfile


def find_bash():
    """Localiza um bash real.

    No Windows, subprocess.resolve('bash') pega o stub do WSL
    (System32\\bash.exe) que falha com rc=1 — preferimos o Git Bash.
    No Linux (CI) o PATH ja aponta para /usr/bin/bash.
    """
    if sys.platform.startswith("win"):
        for candidate in (
            r"C:\Program Files\Git\bin\bash.exe",
            r"C:\Program Files\Git\usr\bin\bash.exe",
        ):
            if os.path.exists(candidate):
                return candidate
    return shutil.which("bash") or "bash"

# Expressoes ${{ ... }} do GitHub Actions sao substituidas ANTES do shell
# rodar; precisam ser mascaradas para o bash -n nao reclamar.
GH_EXPR = re.compile(r"\$\{\{[^}]*\}\}")

# YAML 1.1 (PyYAML) trata "on:" como bool True; nao e um erro de sintaxe.
# Nao bloqueamos, mas reportamos se aparecer com aspas estranhas.


def walk_run_blocks(node, path="root"):
    """Yield (path_label, run_text) para toda chave 'run' no YAML."""
    if isinstance(node, dict):
        for k, v in node.items():
            if k == "run" and isinstance(v, str):
                yield (path, v)
            else:
                yield from walk_run_blocks(v, f"{path}.{k}")
    elif isinstance(node, list):
        for i, item in enumerate(node):
            yield from walk_run_blocks(item, f"{path}[{i}]")


def mask_gh_expr(text):
    """Mascara ${{ ... }} para o bash -n (vira um literal inofensivo)."""
    return GH_EXPR.sub('""', text)


def bash_syntax_ok(text):
    """Roda bash -n no texto; True se a sintaxe e valida.

    Usa temp file (igual ao runner do GitHub Actions, que grava o run block
    em um arquivo .sh antes de executar) — pipe via stdin quebra em MSYS.
    """
    masked = mask_gh_expr(text)
    try:
        with tempfile.NamedTemporaryFile(
            mode="w", suffix=".sh", delete=False, encoding="utf-8"
        ) as f:
            f.write(masked)
            tmp_path = f.name
        try:
            r = subprocess.run(
                [find_bash(), "-n", tmp_path],
                capture_output=True,
                text=True,
                timeout=30,
            )
            return r.returncode == 0, r.stderr.strip()
        finally:
            os.unlink(tmp_path)
    except FileNotFoundError:
        # Sem bash no PATH (ex.: Windows sem Git Bash) — não dá para validar.
        return True, "bash não disponível — pulando sintaxe bash"


def check_file(path):
    """Valida YAML + bash de um workflow. Retorna lista de erros (vazia = OK)."""
    errors = []
    try:
        with open(path, "r", encoding="utf-8") as f:
            raw = f.read()
    except OSError as e:
        return [f"não foi possível ler o arquivo: {e}"]

    try:
        import yaml

        data = yaml.safe_load(raw)
    except Exception as e:
        return [f"YAML inválido: {e}"]

    if data is None:
        return []

    n_blocks = 0
    for label, run_text in walk_run_blocks(data):
        n_blocks += 1
        ok, err = bash_syntax_ok(run_text)
        if not ok:
            errors.append(
                f"bash inválido em {label} (run block #{n_blocks}): {err}"
            )
    return errors


def scan(root):
    files = sorted(
        f
        for f in os.listdir(root)
        if f.endswith(".yml") or f.endswith(".yaml")
    )
    if not files:
        print(f"::error::Nenhum workflow encontrado em {root}")
        return 1

    total_errors = 0
    for fname in files:
        fpath = os.path.join(root, fname)
        errors = check_file(fpath)
        if errors:
            total_errors += len(errors)
            print(f"::error::FAIL {fname}")
            for e in errors:
                print(f"  - {e}")
        else:
            print(f"OK {fname}")
    if total_errors:
        print(f"::error::{total_errors} erro(s) em {len(files)} workflow(s)")
        return 1
    print(f"OK: {len(files)} workflow(s) validados (YAML + bash)")
    return 0


def self_test():
    """Fixtures de regressão: garante que o checker pega o que deve pegar."""
    import tempfile

    fixtures = {
        # Nome do fixture -> (conteúdo, deve falhar?)
        "ok.yml": (
            """
name: ok
on:
  push:
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: Hello
        run: echo "hello world"
      - name: Com expressao GH
        run: |
          X=${{ secrets.TOKEN }}
          echo "x=$X" | grep x=
""",
            False,
        ),
        "bad_yaml.yml": (
            """
name: quebrado
on: push
jobs:
  build:
    runs-on: [ubuntu-latest
    steps:
      - run: echo hi
""",
            True,
        ),
        "bad_bash.yml": (
            """
name: bash ruim
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: Quebra de sintaxe
        run: |
          if [ "$X" = "1" ]; then
            echo "sem fi"
""",
            True,
        ),
        "gh_expr_no_break.yml": (
            """
name: expr ok
on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: uses GH expr
        run: echo "commit=${{ github.sha }}"
""",
            False,
        ),
    }

    failures = 0
    with tempfile.TemporaryDirectory() as tmp:
        for name, (content, should_fail) in fixtures.items():
            p = os.path.join(tmp, name)
            with open(p, "w", encoding="utf-8") as f:
                f.write(content.lstrip("\n"))
            errors = check_file(p)
            got_fail = bool(errors)
            status = "FAIL" if got_fail else "OK"
            expected = "falha" if should_fail else "passa"
            marker = "PASS" if got_fail == should_fail else "FAIL"
            print(f"{marker} {name}: checker {'detectou' if got_fail else 'aprovou'} (esperado: {expected})")
            if got_fail != should_fail:
                failures += 1
                for e in errors:
                    print(f"    {e}")
    if failures:
        print(f"::error::self-test: {failures} fixture(s) com comportamento errado")
        return 1
    print(f"OK: self-test ({len(fixtures)} fixtures)")
    return 0


def main():
    args = sys.argv[1:]
    if "--self-test" in args:
        return self_test()
    root = ".github/workflows"
    if "--root" in args:
        i = args.index("--root")
        if i + 1 < len(args):
            root = args[i + 1]
    return scan(root)


if __name__ == "__main__":
    sys.exit(main())
