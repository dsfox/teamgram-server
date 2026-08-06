// Package queue — клиенты sync и inbox, работающие с очередью или без неё.
//
// Изначально работа этим сервисам передавалась только через Kafka. Очередь даёт
// асинхронность и переживает перезапуск получателя, но на одной машине это лишняя
// движущаяся часть: у обоих сервисов есть тот же приём по gRPC, и порядок апдейтов
// обеспечивается не очередью, а состоянием pts/qts в базе — клиент догоняет
// пропущенное через getDifference.
//
// Выбор по конфигу: заданы Brokers — идём через очередь, иначе прямым вызовом.
package queue

import (
	kafka "github.com/teamgram/marmota/pkg/mq"
	"github.com/teamgram/marmota/pkg/net/rpcx"
	inbox_client "github.com/teamgram/teamgram-server/app/messenger/msg/inbox/client"
	sync_client "github.com/teamgram/teamgram-server/app/messenger/sync/client"

	"github.com/zeromicro/go-zero/zrpc"
)

// Conf — настройки клиента. Поля очереди повторены здесь необязательными:
// в апстримовой структуре Topic и Brokers обязательны, а нам нужно уметь
// обходиться без очереди вовсе.
type Conf struct {
	Topic        string   `json:",optional"`
	Brokers      []string `json:",optional"`
	Username     string   `json:",optional"`
	Password     string   `json:",optional"`
	ProducerAck  string   `json:",optional"`
	CompressType string   `json:",optional"`
	Endpoints    []string `json:",optional"`
}

func (c *Conf) producerConf() *kafka.KafkaProducerConf {
	return &kafka.KafkaProducerConf{
		Topic:        c.Topic,
		Brokers:      c.Brokers,
		Username:     c.Username,
		Password:     c.Password,
		ProducerAck:  c.ProducerAck,
		CompressType: c.CompressType,
	}
}

// UseQueue сообщает, настроена ли очередь.
func (c *Conf) UseQueue() bool {
	return c != nil && len(c.Brokers) > 0
}

// NewSyncClient возвращает клиента sync: через очередь или прямым вызовом.
func NewSyncClient(c *Conf) sync_client.SyncClient {
	if c.UseQueue() {
		return sync_client.NewSyncMqClient(kafka.GetCachedMQClient(c.producerConf()))
	}
	return sync_client.NewSyncClient(rpcClient(c.Endpoints))
}

// NewInboxClient возвращает клиента inbox: через очередь или прямым вызовом.
func NewInboxClient(c *Conf) inbox_client.InboxClient {
	if c.UseQueue() {
		return inbox_client.NewInboxMqClient(kafka.MustKafkaProducer(c.producerConf()))
	}
	return inbox_client.NewInboxClient(rpcClient(c.Endpoints))
}

func rpcClient(endpoints []string) zrpc.Client {
	// NonBlock обязателен: msg обращается к inbox, который живёт в том же процессе,
	// и клиент создаётся раньше, чем поднимется сервер. С ожиданием соединения
	// процесс не стартует вовсе.
	return rpcx.GetCachedRpcClient(zrpc.RpcClientConf{
		Endpoints: endpoints,
		NonBlock:  true,
	})
}
