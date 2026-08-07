"""Достаёт из клиента ключи интерфейса, которые нужно перевести.

Английские строки лежат в клиенте, остальные языки клиент берёт с сервера.
Переводить все 12 тысяч ключей разом незачем: непереведённые клиент показывает
по-английски, поэтому начинаем с того, что видно на каждом экране.

Запуск: python3 server/scripts/extract-langpack-keys.py <Localizable.strings> [префикс ...]
"""
import json
import re
import sys
from pathlib import Path

# Ключ и значение в формате .strings: "Ключ" = "Значение";
LINE = re.compile(r'^"([^"]+)"\s*=\s*"((?:[^"\\]|\\.)*)";\s*$')

# Разделы, которые пользователь видит первым делом: вход, список чатов,
# переписка, настройки, общие кнопки.
DEFAULT_PREFIXES = (
    "Login.", "Localization.", "Common.", "Conversation.", "Chat.", "ChatList.",
    "Settings.", "Notification.", "Contacts.", "Group.", "Profile.", "UserInfo.",
    "Message.", "Media.", "Attachment.", "Compose.", "Weekday.", "Month.",
    "Time.", "Presence.", "Privacy.", "Alert.", "Share.", "Call.", "AccessDenied.",
    "AttachmentMenu.", "PeerInfo.", "DialogList.", "Appearance.", "Passcode.",
    "AuthSessions.", "Web.", "EmptyChat.", "Bot.", "Undo.", "Camera.", "Paint.",
)


def main(path: Path, prefixes):
    strings, plurals = {}, {}

    for line in path.read_text(encoding="utf-8").splitlines():
        match = LINE.match(line.strip())
        if not match:
            continue
        key, value = match.group(1), match.group(2)
        if not key.startswith(tuple(prefixes)):
            continue

        # Формы множественного числа помечены суффиксом: Ключ_one, Ключ_many и т.д.
        parts = key.rsplit("_", 1)
        if len(parts) == 2 and parts[1] in ("zero", "one", "two", "few", "many", "other", "any"):
            base, form = parts
            plurals.setdefault(base, {})[form] = value
        else:
            strings[key] = value

    print(json.dumps(
        {"strings": strings, "plurals": plurals},
        ensure_ascii=False, indent=1, sort_keys=True,
    ))


if __name__ == "__main__":
    if len(sys.argv) < 2:
        raise SystemExit(__doc__)
    main(Path(sys.argv[1]), sys.argv[2:] or DEFAULT_PREFIXES)
