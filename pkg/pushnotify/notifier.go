// Package pushnotify решает, кому из устройств человека отправить уведомление
// о новом сообщении, и отправляет его.
//
// Уведомление уходит только на устройства, где приложение сейчас не открыто:
// если человек читает переписку, сообщение и так придёт по соединению.
package pushnotify

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/pkg/apns"
	"github.com/teamgram/teamgram-server/pkg/devices"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
)

// Notifier — отправка уведомлений о новых сообщениях.
//
// Создаётся всегда, но работает, только если задан ключ Apple. Без ключа все
// вызовы безвредно ничего не делают: сервер остаётся полностью рабочим,
// уведомления просто не приходят.
type Notifier struct {
	registry *devices.Registry
	sender   *apns.Sender
	db       *sqlx.DB
	title    string
	body     string
}

// New собирает отправителя по настройкам из окружения.
func New(db *sqlx.DB) *Notifier {
	n := &Notifier{
		registry: devices.NewRegistry(db),
		db:       db,
		title:    envOr("APNS_TITLE", "2bytes"),
		// Текст сообщения в уведомление не попадает и попасть не может: чтобы
		// показать его в баннере, пришлось бы отправить текст Apple. Здесь
		// только сообщение о том, что что-то пришло.
		body: envOr("APNS_BODY", "Новое сообщение"),
	}

	cfg, ok := apns.ConfigFromEnv()
	if !ok {
		logx.Info("уведомления выключены: ключ Apple не задан (APNS_KEY_PATH и остальные)")
		return n
	}

	sender, err := apns.New(cfg)
	if err != nil {
		logx.Errorf("уведомления выключены: %v", err)
		return n
	}

	n.sender = sender
	logx.Infof("уведомления включены, приложение %s", cfg.Topic)

	return n
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}

	return fallback
}

// Enabled — настроена ли отправка.
func (n *Notifier) Enabled() bool {
	return n != nil && n.sender != nil
}

// NewMessage уведомляет получателя о новом сообщении.
//
// onlineAuthKeyIds — ключи сессий, которым сообщение уже ушло по соединению;
// их устройства пропускаются. peerType/peerId — чат, в который пришло
// сообщение: по ним проверяется, не заглушен ли он.
// Отправка идёт в стороне от доставки сообщения: ответ Apple занимает сотни
// миллисекунд, а при недоступности сети — до минуты. Ждать этого перед тем, как
// показать сообщение остальным устройствам, нельзя.
func (n *Notifier) NewMessage(ctx context.Context, userId int64, peerType int32, peerId int64, onlineAuthKeyIds []int64) {
	if !n.Enabled() {
		return
	}

	threading.GoSafe(func() {
		// Свой контекст: исходный отменится, как только сообщение доставлено.
		sendCtx, cancel := context.WithTimeout(context.Background(), sendTimeout)
		defer cancel()
		n.notify(sendCtx, userId, peerType, peerId, onlineAuthKeyIds)
	})
}

// sendTimeout — сколько всего отводится на уведомление одного человека, включая
// запросы к базе и все его устройства.
const sendTimeout = 30 * time.Second

func (n *Notifier) notify(ctx context.Context, userId int64, peerType int32, peerId int64, onlineAuthKeyIds []int64) {
	if n.muted(ctx, userId, peerType, peerId) {
		return
	}

	list, err := n.registry.ListByUser(ctx, userId)
	if err != nil {
		return
	}

	targets := offlineTargets(list, onlineAuthKeyIds)
	if len(targets) == 0 {
		return
	}

	badge := n.unreadCount(ctx, userId)
	for _, d := range targets {
		n.send(ctx, d, badge)
	}
}

// offlineTargets — устройства, которым нужно уведомление: те, где приложение
// закрыто. Устройству с живой сессией сообщение уже ушло по соединению, и
// уведомление о том, что человек видит на экране, только раздражает.
func offlineTargets(list []devices.DeviceDO, onlineAuthKeyIds []int64) []devices.DeviceDO {
	online := make(map[int64]bool, len(onlineAuthKeyIds))
	for _, id := range onlineAuthKeyIds {
		online[id] = true
	}

	targets := make([]devices.DeviceDO, 0, len(list))
	for _, d := range list {
		if d.IsAPNs() && !online[d.AuthKeyId] {
			targets = append(targets, d)
		}
	}

	return targets
}

func (n *Notifier) send(ctx context.Context, d devices.DeviceDO, badge int) {
	err := n.sender.Send(ctx, d.Token, apns.Notify{
		Title:   n.title,
		Body:    n.body,
		Badge:   badge,
		Sandbox: d.AppSandbox,
	})

	switch {
	case err == nil:
		logx.WithContext(ctx).Infof("уведомление отправлено: пользователь %d, устройство %d", d.UserId, d.AuthKeyId)
	case errors.Is(err, apns.ErrTokenGone):
		logx.WithContext(ctx).Infof("токен устарел, забываем: пользователь %d, устройство %d", d.UserId, d.AuthKeyId)
		_ = n.registry.Forget(ctx, d.TokenType, d.Token)
	default:
		logx.WithContext(ctx).Errorf("уведомление не отправлено: пользователь %d - %v", d.UserId, err)
	}
}

// muted — заглушён ли чат получателем. Настройки заглушения хранит biz-слой,
// но ходить туда по сети ради каждого сообщения дорого, а таблица простая.
func (n *Notifier) muted(ctx context.Context, userId int64, peerType int32, peerId int64) bool {
	var muteUntil int32
	query := "select mute_until from user_notify_settings where user_id = ? and peer_type = ? and peer_id = ? and deleted = 0"

	if err := n.db.QueryRow(ctx, &muteUntil, query, userId, peerType, peerId); err != nil {
		// Записи нет — чат не заглушен, это обычный случай.
		return false
	}

	// -1 означает «настройка не задана», 0 — «звук включён»,
	// большое значение — заглушено надолго.
	return int64(muteUntil) > time.Now().Unix()
}

// unreadCount — сколько непрочитанных всего. Это число iOS ставит на иконку;
// посчитать его больше некому, расширения приложения у нас нет.
func (n *Notifier) unreadCount(ctx context.Context, userId int64) int {
	var total int64
	query := "select coalesce(sum(unread_count), 0) from dialogs where user_id = ? and deleted = 0"

	if err := n.db.QueryRow(ctx, &total, query, userId); err != nil {
		logx.WithContext(ctx).Errorf("не посчитать непрочитанные для %d: %v", userId, err)
		// Отрицательное значение просит iOS не трогать бейдж: лучше оставить
		// как есть, чем показать неверное число.
		return -1
	}

	return int(total)
}
