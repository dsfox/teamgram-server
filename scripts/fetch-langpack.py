"""РАЗОВЫЙ импорт языкового пакета. Постоянной зависимостью не является.

Использован один раз, чтобы не переводить 11 тысяч строк с нуля. Результат лежит
в server/teamgramd/langpack/<код>.json и дальше живёт как наш собственный ресурс:
новые и изменённые строки переводим сами, повторно скрипт не запускаем.

Какие ключи ещё не переведены, показывает server/scripts/langpack-coverage.py.
"""
import asyncio
import json
import re
import sys
from pathlib import Path

from telethon import TelegramClient
from telethon.tl.functions.langpack import GetLangPackRequest
from telethon.tl.types import LangPackStringDeleted, LangPackStringPluralized

ROOT = Path(__file__).resolve().parent.parent
TARGET_DIR = ROOT / "teamgramd" / "langpack"

# Публичный адрес, откуда клиенты забирают языковые пакеты
SOURCE_DC = (2, "149.154.167.50", 443)
SOURCE_API_ID = 8
SOURCE_API_HASH = "7245de8e747a0d6fbe11f7cc14fcc0bb"

APP_NAME = "2bytes"
# Заменяем название приложения, но не трогаем ссылки и адреса вида telegram.org.
# Составные вроде TelegramTips тоже наши — граница слова слева, справа её нет.
BRAND = re.compile(r"\bTelegram(?!\.(?:org|me|dog))")


def rebrand(text: str) -> str:
    return BRAND.sub(APP_NAME, text)


async def main(lang_code: str):
    client = TelegramClient("langpack_fetch", SOURCE_API_ID, SOURCE_API_HASH)
    client.session.set_dc(*SOURCE_DC)
    await client.connect()
    try:
        result = await client(GetLangPackRequest(lang_pack="ios", lang_code=lang_code))
    finally:
        await client.disconnect()

    strings, plurals = {}, {}
    for item in result.strings:
        if isinstance(item, LangPackStringDeleted):
            continue
        if isinstance(item, LangPackStringPluralized):
            forms = {
                name: rebrand(value)
                for name, value in (
                    ("zero", item.zero_value), ("one", item.one_value),
                    ("two", item.two_value), ("few", item.few_value),
                    ("many", item.many_value), ("other", item.other_value),
                )
                if value
            }
            plurals[item.key] = forms
        else:
            strings[item.key] = rebrand(item.value)

    TARGET_DIR.mkdir(parents=True, exist_ok=True)
    target = TARGET_DIR / f"{lang_code}.json"
    target.write_text(json.dumps(
        {"version": result.version, "strings": strings, "plurals": plurals},
        ensure_ascii=False, indent=1, sort_keys=True,
    ), encoding="utf-8")

    print(f"[перевод] {lang_code}: строк {len(strings)}, с множественным числом {len(plurals)}")
    print(f"[перевод] сохранено: {target}")


if __name__ == "__main__":
    asyncio.run(main(sys.argv[1] if len(sys.argv) > 1 else "ru"))
