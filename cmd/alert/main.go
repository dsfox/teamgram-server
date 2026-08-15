// Command alert sends a notification about a server problem to the owner's phone.
//
// The health check finds trouble, but a finding sitting in a log file on the
// server is only useful to someone who goes and looks. This delivers it the same
// way the server delivers messages: through Apple, to the phone.
//
// The owner is named explicitly by ALERT_USER_ID — notifications must never leak
// to whoever happens to have a device registered.
//
// Usage: alert "text of the problem"
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/pkg/apns"
	"github.com/teamgram/teamgram-server/pkg/devices"
)

func parseOwners(raw string) []int64 {
	var owners []int64
	for _, part := range strings.Split(raw, ",") {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err == nil && id != 0 {
			owners = append(owners, id)
		}
	}

	return owners
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: alert \"text of the problem\"")
		os.Exit(2)
	}
	text := strings.Join(os.Args[1:], " ")

	// A comma separated list: the owner may carry more than one phone, and an
	// alert that reaches the one left at home is useless.
	owners := parseOwners(os.Getenv("ALERT_USER_ID"))
	if len(owners) == 0 {
		fmt.Fprintln(os.Stderr, "ALERT_USER_ID is not set: nobody to notify")
		os.Exit(2)
	}

	cfg, ok := apns.ConfigFromEnv()
	if !ok {
		fmt.Fprintln(os.Stderr, "no Apple key configured: cannot send")
		os.Exit(2)
	}
	sender, err := apns.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot use the Apple key: %v\n", err)
		os.Exit(2)
	}

	dsn := os.Getenv("ALERT_MYSQL_DSN")
	if dsn == "" {
		dsn = "teamgram:" + os.Getenv("MYSQL_PASSWORD") + "@tcp(mysql:3306)/teamgram?charset=utf8mb4&parseTime=true"
	}
	registry := devices.NewRegistry(sqlx.NewMySQL(&sqlx.Config{DSN: dsn, Active: 2, Idle: 2}))

	ctx := context.Background()
	var list []devices.DeviceDO
	for _, ownerId := range owners {
		owned, err := registry.ListByUser(ctx, ownerId)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot read devices of %d: %v\n", ownerId, err)
			continue
		}
		list = append(list, owned...)
	}

	sent := 0
	for _, device := range list {
		if !device.IsAPNs() {
			continue
		}
		err := sender.Send(ctx, device.Token, apns.Notify{
			Title: "ice9: server problem",
			Body:  text,
			// A server alert must not touch the unread badge of a chat app
			Badge:   -1,
			Sandbox: device.AppSandbox,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "device %d: %v\n", device.AuthKeyId, err)
			continue
		}
		sent++
	}

	if sent == 0 {
		fmt.Fprintln(os.Stderr, "no device could be reached")
		os.Exit(1)
	}
	fmt.Printf("alert delivered to %d device(s)\n", sent)
}
