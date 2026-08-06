// Package discovery — получение адресов сервиса без обязательного реестра.
//
// Изначально адреса инстансов брались только из etcd. Когда все сервисы живут
// на одной машине и их адреса известны заранее, реестр — лишняя движущаяся часть,
// поэтому здесь поддержаны оба режима: подписка на реестр, если он настроен,
// и прямые адреса из конфига, если нет.
package discovery

import (
	"fmt"

	"github.com/zeromicro/go-zero/core/discov"
)

// Watch вызывает update со списком адресов сервиса.
//
// Если задан реестр — подписывается на изменения и вызывает update при каждом.
// Если реестра нет, а есть прямые адреса — вызывает update один раз: список
// статический, меняться ему неоткуда.
func Watch(etcd discov.EtcdConf, endpoints []string, update func(values []string)) error {
	if len(etcd.Hosts) == 0 {
		if len(endpoints) == 0 {
			return fmt.Errorf("не задан ни реестр (Etcd), ни прямые адреса (Endpoints)")
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
