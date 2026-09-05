// pushrelay holds the Apple and Google keys and forwards notifications for
// servers that have none (#167). A server registers once and then hands it
// pushes that carry no words: the relay's own title and body are the only
// text a phone sees.
//
// Environment:
//
//	PUSH_RELAY_LISTEN   address to serve on, default :8300
//	PUSH_RELAY_DB       the SQLite file for servers and their keys' hashes;
//	                    empty keeps them in memory only
//	PUSH_RELAY_TITLE, PUSH_RELAY_BODY   the words, default "ice9" / "New message"
//	APNS_KEY_PATH, APNS_KEY_ID, APNS_TEAM_ID, APNS_TOPIC   Apple, as ever
//	FCM_SERVICE_ACCOUNT, FCM_PROJECT_ID                    Google, as ever
//
// A platform without keys is served by a forwarder that only says what it
// would send - which is what the local stand runs.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/teamgram/teamgram-server/pkg/apns"
	"github.com/teamgram/teamgram-server/pkg/fcm"
	"github.com/teamgram/teamgram-server/pkg/pushrelay"
)

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	title := envOr("PUSH_RELAY_TITLE", "ice9")
	body := envOr("PUSH_RELAY_BODY", "New message")

	var store pushrelay.Store = pushrelay.NewMemoryStore()
	if path := os.Getenv("PUSH_RELAY_DB"); path != "" {
		opened, err := pushrelay.OpenSQLite(path)
		if err != nil {
			log.Fatalf("cannot open %s: %v", path, err)
		}
		store = opened
		log.Printf("servers kept in %s", path)
	} else {
		log.Printf("servers kept in memory only (PUSH_RELAY_DB is empty)")
	}

	var apple pushrelay.Forwarder = pushrelay.LogOnly{}
	if cfg, ok := apns.ConfigFromEnv(); ok {
		sender, err := apns.New(cfg)
		if err != nil {
			log.Fatalf("apple: %v", err)
		}
		apple = pushrelay.AppleForwarder{Sender: sender, Title: title, Body: body}
		log.Printf("apple notifications enabled for app %s", cfg.Topic)
	} else {
		log.Printf("apple: would send (no key)")
	}

	var google pushrelay.Forwarder = pushrelay.LogOnly{}
	if cfg, ok := fcm.ConfigFromEnv(); ok {
		sender, err := fcm.New(context.Background(), cfg)
		if err != nil {
			log.Fatalf("google: %v", err)
		}
		google = pushrelay.GoogleForwarder{Sender: sender, Title: title, Body: body}
		log.Printf("android notifications enabled for project %s", cfg.ProjectId)
	} else {
		log.Printf("google: would send (no service account)")
	}

	listen := envOr("PUSH_RELAY_LISTEN", ":8300")
	server := &http.Server{
		Addr:              listen,
		Handler:           pushrelay.NewHandler(store, apple, google, pushrelay.NewQuotas(), time.Now),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      35 * time.Second,
	}
	log.Printf("push relay listening on %s", listen)
	log.Fatal(server.ListenAndServe())
}
