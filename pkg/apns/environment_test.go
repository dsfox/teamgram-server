package apns

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ключи Apple бывают привязаны к одной среде: боевой или песочнице. Ключ не той
// среды даёт BadEnvironmentKeyInToken, и уведомления молча не приходят —
// сообщение об этом видно только в логе сервера.
//
// Тест отвечает на вопрос «какие среды обслуживает наш ключ»: отправляет
// заведомо неверный токен устройства в обе.
//
//	BadDeviceToken           — среда работает, забракован только выдуманный токен;
//	BadEnvironmentKeyInToken — ключ для этой среды не годится.
//
// Без ключа тест пропускается.
func TestKeyEnvironments(t *testing.T) {
	keyPath, keyId := findKey(t)

	sender, err := New(Config{
		KeyPath: keyPath,
		KeyId:   envOrDefault("APNS_KEY_ID", keyId),
		TeamId:  envOrDefault("APNS_TEAM_ID", "C3DL5896VG"),
		Topic:   envOrDefault("APNS_TOPIC", "app.twobytes.ios"),
	})
	if err != nil {
		t.Fatalf("ключ не читается: %v", err)
	}
	t.Logf("ключ: %s", filepath.Base(keyPath))

	const fakeToken = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

	for _, c := range []struct {
		name     string
		sandbox  bool
		required bool
	}{
		// Приложение мы раздаём через TestFlight, а такие сборки работают
		// только с боевой средой. Без неё уведомлений не будет ни у кого.
		{"боевая (сборки из TestFlight)", false, true},
		// Песочница нужна лишь для сборок, поставленных на устройство напрямую.
		// Ключ, привязанный к одной среде, её не обслуживает — это не поломка.
		{"песочница (сборки на устройство)", true, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := sender.Send(context.Background(), fakeToken, Notify{
				Title: "проверка", Body: "проверка", Badge: -1, Sandbox: c.sandbox,
			})

			switch {
			case errors.Is(err, ErrTokenGone):
				t.Log("среда работает: ключ принят, забракован только выдуманный токен")
			case err == nil:
				t.Error("выдуманный токен принят — так быть не должно")
			case c.required:
				t.Errorf("среда недоступна, а она обязательна: %v", err)
			default:
				t.Skipf("среда недоступна, для нашей раздачи она и не нужна: %v", err)
			}
		})
	}
}

// findKey ищет ключ в secrets. Имя файла Apple выдаёт в виде
// AuthKey_<идентификатор>.p8, поэтому идентификатор берётся оттуда же —
// хранить его отдельно в коде значило бы держать два источника правды.
func findKey(t *testing.T) (path, keyId string) {
	t.Helper()

	matches, _ := filepath.Glob("../../../secrets/AuthKey_*.p8")
	switch len(matches) {
	case 0:
		t.Skip("ключа в secrets нет, проверять нечего")
	case 1:
	default:
		t.Skipf("ключей несколько (%v) — какой проверять, неочевидно", matches)
	}

	name := filepath.Base(matches[0])
	keyId = strings.TrimSuffix(strings.TrimPrefix(name, "AuthKey_"), ".p8")

	return matches[0], keyId
}

func envOrDefault(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}

	return fallback
}
