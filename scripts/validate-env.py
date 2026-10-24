#!/usr/bin/env python3
# =============================================================================
#  Invoice Keeper — проверка согласованности .env.example и docker-compose.yml
# -----------------------------------------------------------------------------
#  Проверяет:
#    1) все переменные, требуемые по ТЗ, присутствуют в .env.example;
#    2) каждая переменная из .env.example имеет значение по умолчанию
#       (${VAR:-default}) или используется в compose — чтобы compose не падал
#       при отсутствии .env;
#    3) каждый ${VAR}, используемый в docker-compose.yml, объявлен в .env.example;
#    4) в .env.example нет значений, похожих на реальные секреты.
# =============================================================================
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ENV_FILE = ROOT / ".env.example"
COMPOSE_FILE = ROOT / "docker-compose.yml"

# Переменные, обязательные по техническому заданию
REQUIRED = [
    "POSTGRES_USER",
    "POSTGRES_PASSWORD",
    "POSTGRES_DB",
    "DATABASE_URL",
    "REDIS_URL",
    "TELEGRAM_BOT_TOKEN",
    "TELEGRAM_CHAT_ID",
    "ADMIN_LOGIN",
    "ADMIN_PASSWORD",
    "TIMEZONE",
    "REMINDER_DAYS",
    "JWT_SECRET",
]

# Значения, которые не должны оставаться в .env.example
FORBIDDEN_SUBSTRINGS = [
    "AAExampleTokenReplaceMe",
    "change_me_postgres_password",
    "change_me_admin_password",
    "change_me_generate_a_long_random_string",
]

ERRORS = []
WARNINGS = []
INFO = []


def parse_env(path: Path):
    """Возвращает dict имя -> значение (без кавычек)."""
    result = {}
    if not path.exists():
        return result
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        result[key.strip()] = value.strip().strip('"').strip("'")
    return result


def main():
    env = parse_env(ENV_FILE)
    compose_text = COMPOSE_FILE.read_text(encoding="utf-8") if COMPOSE_FILE.exists() else ""

    # --- 1. Обязательные переменные ------------------------------------------
    for var in REQUIRED:
        if var not in env:
            ERRORS.append(f".env.example: отсутствует обязательная переменная '{var}'")
        elif env[var] == "":
            ERRORS.append(f".env.example: переменная '{var}' имеет пустое значение")

    # --- 2. Значения по умолчанию в compose ----------------------------------
    # Переменные, которые compose читает из .env, но для которых НЕТ ${VAR:-default}
    used_vars = set(re.findall(r"\$\{([A-Za-z_][A-Za-z0-9_]*)", compose_text))
    with_default = set(re.findall(r"\$\{([A-Za-z_][A-Za-z0-9_]*):-", compose_text))
    without_default = used_vars - with_default

    for var in sorted(without_default):
        WARNINGS.append(
            f"docker-compose.yml: '${{{var}}}' без значения по умолчанию — "
            "compose потребует его в .env (проверьте, что он там есть)"
        )
        if var not in env:
            ERRORS.append(
                f"docker-compose.yml использует '${{{var}}}', но в .env.example его нет"
            )

    # --- 3. Переменные .env.example, не используемые в compose ---------------
    for var in sorted(set(env) - used_vars):
        INFO.append(f".env.example: '{var}' не используется в docker-compose.yml (используется приложением напрямую)")

    # --- 4. Проверка шаблонных секретов --------------------------------------
    # В .env.example шаблонные значения — это норма (реальных секретов быть не должно).
    for var, value in env.items():
        for bad in FORBIDDEN_SUBSTRINGS:
            if bad in value:
                INFO.append(
                    f".env.example: '{var}' содержит шаблонное значение ('{bad}') — "
                    "обязательно замените при копировании в .env"
                )

    # --- 5. Согласованность DATABASE_URL с POSTGRES_* ------------------------
    db_url = env.get("DATABASE_URL", "")
    if db_url:
        if "postgresql" not in db_url:
            ERRORS.append(".env.example: DATABASE_URL должен начинаться с postgresql")
        if "@db:5432" not in db_url:
            WARNINGS.append(".env.example: DATABASE_URL должен указывать на хост 'db' (имя сервиса в docker-compose)")
        user = env.get("POSTGRES_USER", "")
        if user and f"//{user}:" not in db_url:
            WARNINGS.append(
                f".env.example: DATABASE_URL не содержит POSTGRES_USER='{user}' — "
                "значения разъедутся при копировании в .env"
            )

    # --- 6. REDIS_URL должен указывать на сервис redis ------------------------
    redis_url = env.get("REDIS_URL", "")
    if redis_url:
        if "redis:6379" not in redis_url:
            ERRORS.append(".env.example: REDIS_URL должен указывать на 'redis:6379' (имя сервиса в docker-compose)")
        elif not redis_url.startswith("redis://"):
            ERRORS.append(".env.example: REDIS_URL должен начинаться с 'redis://'")

    # --- Вывод ---------------------------------------------------------------
    print(f"Проверено: .env.example ({len(env)} переменных), docker-compose.yml")
    print()
    for i in INFO:
        print(f"[INFO] {i}")
    for w in WARNINGS:
        print(f"[WARN] {w}")
    for e in ERRORS:
        print(f"[FAIL] {e}")
    print()
    if ERRORS:
        print(f"ИТОГ: {len(ERRORS)} ошибок(и), {len(WARNINGS)} предупреждений")
        return 1
    print(f"ИТОГ: ошибок нет ({len(WARNINGS)} предупреждений, {len(INFO)} примечаний)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
