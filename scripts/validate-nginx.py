#!/usr/bin/env python3
# =============================================================================
#  Invoice Keeper — структурная проверка nginx-конфигов
#  (используется, когда нет доступа к Docker Hub для запуска `nginx -t`)
#
#  Проверяет:
#    * баланс скобок { } ;
#    * наличие обязательных директив: events, http, server, listen, upstream;
#    * что все location-блоки вложены в server;
#    * что upstream'ы объявлены и используются;
#    * что proxy_pass ссылается на существующий upstream;
#    * корректность регулярных выражений в location;
#    * что server-level директивы не стоят внутри location.
# =============================================================================
import re
import sys
from pathlib import Path

ERRORS = []
WARNINGS = []


def strip_comments(text: str) -> str:
    """Убирает # комментарии, не трогая # внутри кавычек."""
    out = []
    for line in text.splitlines():
        result, in_s, in_d = [], False, False
        for ch in line:
            if in_s:
                result.append(ch)
                if ch == "'":
                    in_s = False
            elif in_d:
                result.append(ch)
                if ch == '"':
                    in_d = False
            else:
                if ch == "'":
                    in_s = True
                    result.append(ch)
                elif ch == '"':
                    in_d = True
                    result.append(ch)
                elif ch == '#':
                    break
                else:
                    result.append(ch)
        out.append("".join(result))
    return "\n".join(out)


def find_blocks(text: str, keyword: str):
    """Возвращает список (start, end, header) для блоков keyword { ... }.

    Учитывает вложенность и пропускает совпадения вида "server backend:8000;".
    """
    blocks = []
    for m in re.finditer(rf"\b{keyword}\s*(?:\(|~\s*)?[^\;{{]*\{{", text):
        header = m.group(0)[:-1].strip()
        # ищем соответствующую закрывающую скобку
        depth, i = 0, m.end() - 1
        while i < len(text):
            if text[i] == '{':
                depth += 1
            elif text[i] == '}':
                depth -= 1
                if depth == 0:
                    blocks.append((m.start(), i, header))
                    break
            i += 1
    return blocks


def check_balance(text: str, name: str):
    depth, line_no = 0, 1
    for ch in text:
        if ch == '\n':
            line_no += 1
        elif ch == '{':
            depth += 1
        elif ch == '}':
            depth -= 1
            if depth < 0:
                ERRORS.append(f"{name}: лишняя '}}' около строки {line_no}")
                return
    if depth != 0:
        ERRORS.append(f"{name}: незакрытых блоков — {depth} (ожидается 0)")


def check_directives(text: str, name: str):
    required = {
        "nginx.conf": ["events", "http", "server", "listen", "upstream"],
        "frontend.conf": ["server", "listen", "root", "location"],
    }
    for d in required.get(name, []):
        if not re.search(rf"\b{d}\b", text):
            ERRORS.append(f"{name}: отсутствует обязательная директива '{d}'")


def check_upstreams(text: str, name: str):
    ups = set(re.findall(r"upstream\s+([A-Za-z0-9_]+)\s*\{", text))
    used = set(re.findall(r"proxy_pass\s+http://([A-Za-z0-9_]+)", text))
    for u in used:
        if u not in ups:
            ERRORS.append(f"{name}: proxy_pass ссылается на необъявленный upstream '{u}'")
    for u in ups:
        if u not in used:
            WARNINGS.append(f"{name}: upstream '{u}' объявлен, но не используется")
    for start, end, header in find_blocks(text, "upstream"):
        body = text[start:end]
        if not re.search(r"\bserver\s+[\w.-]+:\d+", body):
            ERRORS.append(f"{name}: в '{header}' нет ни одного server host:port")


def check_locations(text: str, name: str):
    server_blocks = find_blocks(text, "server")
    loc_blocks = find_blocks(text, "location")

    for start, end, header in loc_blocks:
        inside = any(s < start < e for s, e, _ in server_blocks)
        if not inside:
            ERRORS.append(f"{name}: location '{header}' находится вне блока server")

    # Регулярные выражения должны компилироваться
    for m in re.finditer(r"location\s+~\s*(\S+)\s*\{", text):
        pattern = m.group(1)
        try:
            re.compile(pattern)
        except re.error as e:
            ERRORS.append(f"{name}: некорректное regex в location: {pattern} ({e})")

    # Директивы server-level недопустимы внутри location
    for start, end, header in loc_blocks:
        body = text[start:end]
        for d in ("listen", "server_name", "root", "upstream", "user", "worker_processes"):
            if re.search(rf"^\s*{d}\s+", body, re.M):
                ERRORS.append(f"{name}: директива '{d}' недопустима внутри location '{header}'")

    # Каждый location должен содержать proxy_pass или try_files/return
    for start, end, header in loc_blocks:
        body = text[start:end]
        if not re.search(r"\b(proxy_pass|try_files|return|root)\b", body):
            WARNINGS.append(f"{name}: location '{header}' не содержит proxy_pass/try_files/return")


def check_proxy_targets(text: str, name: str):
    """Каждый location с proxy_pass обязан использовать http:// (не裸 upstream)."""
    for m in re.finditer(r"proxy_pass\s+([^;]+);", text):
        target = m.group(1).strip()
        if not target.startswith("http://"):
            ERRORS.append(f"{name}: proxy_pass '{target}' должен начинаться с http://")


def check_servers_listen(text: str, name: str):
    for start, end, header in find_blocks(text, "server"):
        body = text[start:end]
        if not re.search(r"\blisten\s+", body):
            ERRORS.append(f"{name}: server-блок '{header}' без директивы listen")
        if "location" not in body:
            WARNINGS.append(f"{name}: server-блок '{header}' не содержит ни одного location")


def main():
    root = Path(__file__).resolve().parent.parent
    targets = [
        root / "infra" / "nginx" / "nginx.conf",
        root / "infra" / "nginx" / "frontend.conf",
    ]

    for path in targets:
        if not path.exists():
            ERRORS.append(f"Файл не найден: {path}")
            continue
        raw = path.read_text(encoding="utf-8")
        text = strip_comments(raw)

        check_balance(text, path.name)
        check_directives(text, path.name)
        check_upstreams(text, path.name)
        check_locations(text, path.name)
        check_proxy_targets(text, path.name)
        check_servers_listen(text, path.name)

        print(f"[OK] проверен {path.relative_to(root)} ({len(raw.splitlines())} строк)")

    print()
    for w in WARNINGS:
        print(f"[WARN] {w}")
    for e in ERRORS:
        print(f"[FAIL] {e}")

    print()
    if ERRORS:
        print(f"ИТОГ: {len(ERRORS)} ошибок(и), {len(WARNINGS)} предупреждений")
        return 1
    print(f"ИТОГ: структурных ошибок нет ({len(WARNINGS)} предупреждений)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
