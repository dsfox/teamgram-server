// Package apns отправляет уведомления на устройства Apple.
//
// Важное свойство: в уведомление не попадает ни текст сообщения, ни имя
// отправителя — только «Новое сообщение». Это осознанный выбор: показать текст
// в баннере можно лишь передав его Apple, пусть и зашифрованным. Мы не передаём
// ничего — сервер Apple видит только факт события. Расшифровкой и подстановкой
// текста в Telegram занимается отдельное расширение приложения, которого у нас
// нет; см. docs/03-push-uvedomleniya.md.
package apns

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/payload"
	"github.com/sideshow/apns2/token"
	"golang.org/x/net/http2"
)

// ErrTokenGone — Apple сообщила, что токен больше не существует: приложение
// удалено с устройства. Такой токен нужно забыть, повторять отправку бесполезно.
var ErrTokenGone = errors.New("apns: device token is no longer valid")

// Config — то, что выдаёт портал разработчика Apple. Ключ бессрочный и один
// на все приложения команды.
type Config struct {
	KeyPath string // файл .p8 с закрытым ключом
	KeyId   string // идентификатор ключа, 10 символов
	TeamId  string // идентификатор команды, 10 символов
	Topic   string // идентификатор приложения, он же bundle id
}

// ConfigFromEnv читает настройки из окружения. Второе значение — заданы ли они
// вообще: без ключа сервер работает как раньше, просто молча не отправляет
// уведомления.
func ConfigFromEnv() (Config, bool) {
	c := Config{
		KeyPath: os.Getenv("APNS_KEY_PATH"),
		KeyId:   os.Getenv("APNS_KEY_ID"),
		TeamId:  os.Getenv("APNS_TEAM_ID"),
		Topic:   os.Getenv("APNS_TOPIC"),
	}

	return c, c.KeyPath != "" && c.KeyId != "" && c.TeamId != "" && c.Topic != ""
}

// Sender отправляет уведомления. Держит два подключения: боевое и к песочнице.
// Смешивать их нельзя — токен, выданный устройству в одной среде, в другой
// недействителен, поэтому среду выбирает вызывающий по данным устройства.
type Sender struct {
	topic      string
	production *apns2.Client
	sandbox    *apns2.Client
}

func New(c Config) (*Sender, error) {
	authKey, err := token.AuthKeyFromFile(c.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("apns: не прочитать ключ %s: %w", c.KeyPath, err)
	}

	tok := &token.Token{
		AuthKey: authKey,
		KeyID:   c.KeyId,
		TeamID:  c.TeamId,
	}

	return &Sender{
		topic:      c.Topic,
		production: newClient(tok).Production(),
		sandbox:    newClient(tok).Development(),
	}, nil
}

// newClient повторяет настройки apns2 по умолчанию, добавляя ограничение на
// время жизни простаивающего соединения. Без него число горутин растёт:
// соединения к Apple копятся и не закрываются (sideshow/apns2#238).
func newClient(tok *token.Token) *apns2.Client {
	transport := &http2.Transport{
		ReadIdleTimeout: apns2.ReadIdleTimeout,
	}

	client := apns2.NewTokenClient(tok)
	client.HTTPClient = &http.Client{
		Transport: transport,
		Timeout:   apns2.HTTPClientTimeout,
	}

	return client
}

// Notify — уведомление о новом событии для одного устройства.
//
// Badge — число непрочитанных; iOS ставит его на иконку как есть. Отрицательное
// значение означает «не трогать бейдж».
type Notify struct {
	Title   string
	Body    string
	Badge   int
	Sandbox bool
}

// Send доставляет уведомление. Возвращает ErrTokenGone, если токен пора забыть.
func (s *Sender) Send(ctx context.Context, deviceToken string, n Notify) error {
	p := payload.NewPayload().
		AlertTitle(n.Title).
		AlertBody(n.Body).
		Sound("default")
	if n.Badge >= 0 {
		p = p.Badge(n.Badge)
	}

	client := s.production
	if n.Sandbox {
		client = s.sandbox
	}

	res, err := client.PushWithContext(ctx, &apns2.Notification{
		DeviceToken: deviceToken,
		Topic:       s.topic,
		Payload:     p,
		Priority:    apns2.PriorityHigh,
		PushType:    apns2.PushTypeAlert,
		// Уведомление о непрочитанном сообщении устаревает: если телефон был
		// выключен сутки, показывать его при включении незачем — человек и так
		// увидит переписку.
		Expiration: time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		return fmt.Errorf("apns: отправка не удалась: %w", err)
	}

	switch res.Reason {
	case apns2.ReasonUnregistered, apns2.ReasonBadDeviceToken, apns2.ReasonExpiredToken:
		return ErrTokenGone
	}

	if !res.Sent() {
		return fmt.Errorf("apns: отказ %d %s", res.StatusCode, res.Reason)
	}

	return nil
}
