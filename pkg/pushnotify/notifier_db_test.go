package pushnotify

import (
	"context"
	"os"
	"testing"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/pkg/devices"
)

// Чтение из базы иначе нигде не проверяется: сценарии сверяют запись токена,
// а читает его код отправки, который без ключа Apple не запускается. Здесь
// проверяется именно чтение — типы колонок легко разъезжаются с Go-структурой
// и обнаруживаются только в бою.
//
// Тест требует поднятого стенда (deploy/docker-compose.yml). Без него молча
// пропускается, чтобы не ломать сборку там, где базы нет.
const testDSN = "teamgram:teamgram@tcp(127.0.0.1:3306)/teamgram?charset=utf8mb4&parseTime=true"

func openTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	dsn := testDSN
	if v := os.Getenv("TEST_MYSQL_DSN"); v != "" {
		dsn = v
	}

	db := sqlx.NewMySQL(&sqlx.Config{DSN: dsn, Active: 2, Idle: 2})
	if err := db.QueryRow(context.Background(), new(int), "select 1"); err != nil {
		t.Skipf("база недоступна, тест пропущен: %v", err)
	}

	return db
}

func TestReadDeviceBack(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	registry := devices.NewRegistry(db)

	const userId = -424242 // заведомо несуществующий человек, чтобы не мешать данным
	want := devices.DeviceDO{
		AuthKeyId:  -1,
		UserId:     userId,
		TokenType:  devices.TokenTypeAPNs,
		Token:      "проверка чтения",
		NoMuted:    true,
		AppSandbox: true,
		Secret:     "00ff",
		OtherUids:  "1,2",
	}

	if err := registry.Register(ctx, &want); err != nil {
		t.Fatalf("не записать устройство: %v", err)
	}
	t.Cleanup(func() {
		_ = registry.Unregister(ctx, want.AuthKeyId, want.UserId, want.TokenType, want.Token)
	})

	list, err := registry.ListByUser(ctx, userId)
	if err != nil {
		t.Fatalf("не прочитать устройства: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ожидалось одно устройство, получено %d", len(list))
	}

	got := list[0]
	switch {
	case got.Token != want.Token:
		t.Errorf("токен: %q вместо %q", got.Token, want.Token)
	case got.TokenType != want.TokenType:
		t.Errorf("тип токена: %d вместо %d", got.TokenType, want.TokenType)
	case !got.AppSandbox:
		t.Error("потеряна пометка о песочнице: уведомление уйдёт не на тот сервер Apple")
	case !got.NoMuted:
		t.Error("потерян признак no_muted")
	case got.Secret != want.Secret:
		t.Errorf("секрет: %q вместо %q", got.Secret, want.Secret)
	case !got.IsAPNs():
		t.Error("устройство не опознано как пригодное для уведомлений")
	}
}

func TestUnreadCountReadsAsNumber(t *testing.T) {
	db := openTestDB(t)

	// sum() в MySQL возвращает decimal, а не целое: если чтение написано
	// неаккуратно, здесь будет ошибка преобразования типа.
	n := &Notifier{db: db}
	ctx := context.Background()

	if got := n.unreadCount(ctx, -424242); got != 0 {
		t.Fatalf("для человека без переписок ожидался 0, получено %d", got)
	}

	// Пустая выборка отдаёт ноль из coalesce и ничего не доказывает про типы.
	// Настоящая проверка — на человеке с непрочитанными.
	var userId int64
	err := db.QueryRow(ctx, &userId,
		"select user_id from dialogs where unread_count > 0 and deleted = 0 limit 1")
	if err != nil {
		t.Skip("в базе нет непрочитанных сообщений, проверку типа делать не на чем")
	}

	if got := n.unreadCount(ctx, userId); got <= 0 {
		t.Fatalf("у человека есть непрочитанные, а посчитано %d", got)
	}
}

func TestMutedForUnknownPeer(t *testing.T) {
	db := openTestDB(t)

	// Настроек нет — чат не заглушен. Обратный ответ означал бы, что уведомления
	// молча не приходят никому.
	n := &Notifier{db: db}
	if n.muted(context.Background(), -424242, 2, -424243) {
		t.Fatal("чат без настроек считается заглушенным")
	}
}
