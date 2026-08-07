// Package discovery resolves service addresses without requiring a registry.
//
// Originally instance addresses came from etcd only. When every service lives on
// one machine and the addresses are known in advance, a registry is one moving
// part too many, so both modes are supported here: subscribing to the registry
// when it is configured, and direct addresses from the config when it is not.
package discovery

import (
	"fmt"

	"github.com/zeromicro/go-zero/core/discov"
)

// Watch calls update with the list of service addresses.
//
// With a registry configured it subscribes to changes and calls update on each
// one. Without a registry but with direct addresses it calls update once: the
// list is static and has no source of change.
func Watch(etcd discov.EtcdConf, endpoints []string, update func(values []string)) error {
	if len(etcd.Hosts) == 0 {
		if len(endpoints) == 0 {
			return fmt.Errorf("neither a registry (Etcd) nor direct addresses (Endpoints) are configured")
		}
		update(endpoints)
		return nil
	}

	subscriber, err := discov.NewSubscriber(etcd.Hosts, etcd.Key)
	if err != nil {
		return err
	}

	listener := func() {
		update(subscriber.Values())
	}
	subscriber.AddListener(listener)
	listener()

	return nil
}
