package pushnotify

import (
	"context"
	"os"
	"time"

	"github.com/teamgram/teamgram-server/pkg/pushrelay"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/threading"
)

// withRelay turns notifications on, through the relay PUSH_RELAY_URL names.
//
// The relay holds the Apple and Google keys; this server holds a key the
// relay gave it, in its own database. A server that has none yet registers
// itself - now, and again until the relay answers - so that a server put up
// with the one line, on a machine that could not reach the relay at that
// moment, wakes phones from the first minute it can.
func (n *Notifier) withRelay() *Notifier {
	url := os.Getenv("PUSH_RELAY_URL")
	if url == "" {
		logx.Info("notifications disabled: no relay configured (PUSH_RELAY_URL)")
		return n
	}
	n.relayURL = url

	var row PushRelayDO
	err := n.db.QueryRow(context.Background(), &row,
		"select url, server_id, relay_key, registered_at from push_relay where url = ?", url)
	if err == nil && row.RelayKey != "" {
		n.relay.Store(&pushrelay.Client{URL: url, Key: row.RelayKey})
		logx.Infof("notifications go through the relay %s as %s", url, row.ServerId)
		return n
	}

	logx.Infof("notifications wait for the relay %s", url)
	threading.GoSafe(func() { n.register(url) })
	return n
}

// register keeps asking until the relay answers: at once, then soon, then
// every ten minutes.
func (n *Notifier) register(url string) {
	waits := []time.Duration{0, 5 * time.Second, 30 * time.Second}
	for attempt := 0; ; attempt++ {
		wait := 10 * time.Minute
		if attempt < len(waits) {
			wait = waits[attempt]
		}
		time.Sleep(wait)
		if n.relay.Load() != nil {
			return
		}
		address := os.Getenv("ICE9_ADDRESS")
		if address == "" {
			address = "unnamed"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		id, key, err := pushrelay.Register(ctx, url, address)
		cancel()
		if err != nil {
			logx.Infof("the relay %s did not register this server: %v", url, err)
			continue
		}
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		_, err = n.db.Exec(ctx, "insert into push_relay (url, server_id, relay_key, registered_at) values (?, ?, ?, ?)",
			url, id, key, time.Now().Unix())
		cancel()
		if err != nil {
			logx.Errorf("registered with the relay %s as %s, but could not keep the key: %v", url, id, err)
			continue
		}
		n.relay.Store(&pushrelay.Client{URL: url, Key: key})
		logx.Infof("registered with the relay %s as %s", url, id)
		return
	}
}
