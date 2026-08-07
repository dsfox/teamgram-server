"""Показывает, какие строки интерфейса ещё не переведены.

Рабочий инструмент локализации: языковой пакет — наш собственный ресурс,
и новые ключи (свои или пришедшие с обновлением клиента) переводим сами.
Скрипт сверяет ключи английского файла клиента с нашим переводом.

Запуск:
    python3 server/scripts/langpack-coverage.py ru                 сводка
    python3 server/scripts/langpack-coverage.py ru --missing       список непереведённых
    python3 server/scripts/langpack-coverage.py ru --export ФАЙЛ   выгрузить их для перевода
"""
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
CLIENT_STRINGS = ROOT.parent / "clients/ios/Telegram/Telegram-iOS/en.lproj/Localizable.strings"
LANGPACK_DIR = ROOT / "teamgramd" / "langpack"

LINE = re.compile(r'^"([^"]+)"\s*=\s*"((?:[^"\\]|\\.)*)";\s*$')

# Клиент хранит формы множественного числа с суффиксами _0/_1/_2/_3_10/_many/_any
# (см. getPluralizationSuffix в build-system/GenerateStrings), а в языковом пакете
# они лежат под именами CLDR. Без этого соответствия переведённые ключи выглядят
# непереведёнными и объём работы кажется больше, чем он есть.
PLURAL_SUFFIX = ("_0", "_1", "_2", "_3_10", "_many", "_any",
                 "_zero", "_one", "_two", "_few", "_other")


def client_keys() -> dict:
    keys = {}
    for line in CLIENT_STRINGS.read_text(encoding="utf-8").splitlines():
        match = LINE.match(line.strip())
        if not match:
            continue
        key, value = match.group(1), match.group(2)
        # формы множественного числа сводим к базовому ключу
        for suffix in PLURAL_SUFFIX:
            if key.endswith(suffix):
                key = key[: -len(suffix)]
                break
        keys.setdefault(key, value)
    return keys


def main(lang_code: str, mode: str, export: str = None):
    pack = json.loads((LANGPACK_DIR / f"{lang_code}.json").read_text(encoding="utf-8"))
    translated = set(pack["strings"]) | set(pack["plurals"])
    source = client_keys()

    missing = {key: value for key, value in source.items() if key not in translated}
    covered = len(source) - len(missing)
    percent = covered * 100 // max(1, len(source))

    print(f"ключей в клиенте: {len(source)}")
    print(f"переведено:       {covered} ({percent}%)")
    print(f"не переведено:    {len(missing)}")

    if mode == "--missing":
        for key, value in sorted(missing.items())[:80]:
            print(f"  {key} = {value[:70]}")
        if len(missing) > 80:
            print(f"  ... и ещё {len(missing) - 80}")
    elif mode == "--export" and export:
        Path(export).write_text(
            json.dumps(missing, ensure_ascii=False, indent=1, sort_keys=True), encoding="utf-8")
        print(f"выгружено для перевода: {export}")


if __name__ == "__main__":
    if len(sys.argv) < 2:
        raise SystemExit(__doc__)
    main(sys.argv[1], sys.argv[2] if len(sys.argv) > 2 else "",
         sys.argv[3] if len(sys.argv) > 3 else None)
