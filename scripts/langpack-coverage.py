"""Shows which interface strings are still untranslated.

A working localisation tool: the language pack is our own resource, and new keys
(ours or arriving with a client update) are translated by us. The script compares
the keys of the client English file with our translation.

Usage:
    python3 server/scripts/langpack-coverage.py ru                 summary
    python3 server/scripts/langpack-coverage.py ru --missing       list the untranslated ones
    python3 server/scripts/langpack-coverage.py ru --export FILE   export them for translation
"""
import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
CLIENT_STRINGS = ROOT.parent / "clients/ios/Telegram/Telegram-iOS/en.lproj/Localizable.strings"
LANGPACK_DIR = ROOT / "teamgramd" / "langpack"

LINE = re.compile(r'^"([^"]+)"\s*=\s*"((?:[^"\\]|\\.)*)";\s*$')

# The client stores plural forms with the suffixes _0/_1/_2/_3_10/_many/_any
# (see getPluralizationSuffix in build-system/GenerateStrings), while the language
# pack keeps them under CLDR names. Without this mapping translated keys look
# untranslated and the work appears larger than it is.
PLURAL_SUFFIX = ("_0", "_1", "_2", "_3_10", "_many", "_any",
                 "_zero", "_one", "_two", "_few", "_other")


def client_keys() -> dict:
    keys = {}
    for line in CLIENT_STRINGS.read_text(encoding="utf-8").splitlines():
        match = LINE.match(line.strip())
        if not match:
            continue
        key, value = match.group(1), match.group(2)
        # plural forms collapse to their base key
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

    print(f"keys in the client:  {len(source)}")
    print(f"translated:         {covered} ({percent}%)")
    print(f"untranslated:       {len(missing)}")

    if mode == "--missing":
        for key, value in sorted(missing.items())[:80]:
            print(f"  {key} = {value[:70]}")
        if len(missing) > 80:
            print(f"  ... and {len(missing) - 80} more")
    elif mode == "--export" and export:
        Path(export).write_text(
            json.dumps(missing, ensure_ascii=False, indent=1, sort_keys=True), encoding="utf-8")
        print(f"exported for translation: {export}")


if __name__ == "__main__":
    if len(sys.argv) < 2:
        raise SystemExit(__doc__)
    main(sys.argv[1], sys.argv[2] if len(sys.argv) > 2 else "",
         sys.argv[3] if len(sys.argv) > 3 else None)
