"""A ONE-OFF language pack import. It is not a permanent dependency.

    python3 server/scripts/fetch-langpack.py ru ios
    python3 server/scripts/fetch-langpack.py ru android

The platform matters: the two clients name the same sentence differently, and a
pack fetched for one is a pack of keys the other has never heard of. Serving the
iOS pack to Android is how "choosing Russian does nothing" happened - eleven
thousand strings arrived and not one of them matched.

Used once so that eleven thousand strings did not have to be translated from
scratch. The result lives in server/teamgramd/langpack/<code>.json and from then
on is our own resource: new and changed strings are translated by us and the
script is never run again.

Which keys are still untranslated is shown by server/scripts/langpack-coverage.py.
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

# The public address clients fetch language packs from
SOURCE_DC = (2, "149.154.167.50", 443)
SOURCE_API_ID = 8
SOURCE_API_HASH = "7245de8e747a0d6fbe11f7cc14fcc0bb"

APP_NAME = "2bytes"
# Replace the app name but leave links and addresses such as telegram.org alone.
# Compounds like TelegramTips are ours too: a word boundary on the left, none on the right.
BRAND = re.compile(r"\bTelegram(?!\.(?:org|me|dog))")


def rebrand(text: str) -> str:
    return BRAND.sub(APP_NAME, text)


async def main(lang_code: str, platform: str):
    client = TelegramClient("langpack_fetch", SOURCE_API_ID, SOURCE_API_HASH)
    client.session.set_dc(*SOURCE_DC)
    await client.connect()
    try:
        result = await client(GetLangPackRequest(lang_pack=platform, lang_code=lang_code))
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
    # The iOS pack keeps the plain name it has always had; anything else is
    # suffixed, so the two live side by side.
    name = lang_code if platform == "ios" else f"{lang_code}.{platform}"
    target = TARGET_DIR / f"{name}.json"
    target.write_text(json.dumps(
        {"version": result.version, "strings": strings, "plurals": plurals},
        ensure_ascii=False, indent=1, sort_keys=True,
    ), encoding="utf-8")

    print(f"[translation] {lang_code}/{platform}: {len(strings)} strings, {len(plurals)} pluralised")
    print(f"[translation] saved: {target}")


if __name__ == "__main__":
    asyncio.run(main(
        sys.argv[1] if len(sys.argv) > 1 else "ru",
        sys.argv[2] if len(sys.argv) > 2 else "ios",
    ))
