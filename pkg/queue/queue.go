// Package queue holds the sync and inbox clients, working with or without a queue.
//
// Originally work reached these services through Kafka only. A queue gives
// asynchrony and survives a restart of the consumer, but on a single machine it
// is one moving part too many: both services accept the same work over gRPC, and
// update ordering rests on the pts/qts state in the database rather than on the
// queue — the client catches up through getDifference.
//
// The config decides: with Brokers set we go through the queue, otherwise we call directly.
package queue

import (
	kafka "github.com/teamgram/marmota/pkg/mq"
	"github.com/teamgram/marmota/pkg/net/rpcx"
	inbox_client "github.com/teamgram/teamgram-server/app/messenger/msg/inbox/client"
	sync_client "github.com/teamgram/teamgram-server/app/messenger/sync/client"

	"github.com/zeromicro/go-zero/zrpc"
)

// Conf holds the client settings. The queue fields are repeated here as
// optional: in the upstream struct Topic and Brokers are mandatory, while we
// need to work without a queue at all.
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

// UseQueue reports whether a queue is configured.
func (c *Conf) UseQueue() bool {
	return c != nil && len(c.Brokers) > 0
}

// NewSyncClient returns a sync client: through the queue or by a direct call.
func NewSyncClient(c *Conf) sync_client.SyncClient {
	if c.UseQueue() {
		return sync_client.NewSyncMqClient(kafka.GetCachedMQClient(c.producerConf()))
	}
	return sync_client.NewSyncClient(rpcClient(c.Endpoints))
}

// NewInboxClient returns an inbox client: through the queue or by a direct call.
func NewInboxClient(c *Conf) inbox_client.InboxClient {
	if c.UseQueue() {
		return inbox_client.NewInboxMqClient(kafka.MustKafkaProducer(c.producerConf()))
	}
	return inbox_client.NewInboxClient(rpcClient(c.Endpoints))
}

func rpcClient(endpoints []string) zrpc.Client {
	// NonBlock is mandatory: msg talks to inbox, which lives in the same process,
	// and the client is created before the server comes up. Waiting for the
	// connection would keep the process from starting at all.
	return rpcx.GetCachedRpcClient(zrpc.RpcClientConf{
		Endpoints: endpoints,
		NonBlock:  true,
	})
}
