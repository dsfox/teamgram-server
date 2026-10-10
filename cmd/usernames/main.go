// Command usernames gives a username to every account that has none (#239).
//
// Sign-up gives one since #239; the accounts made before it had none unless
// the person set one by hand, and such an account is mentioned in a group by
// whatever name the one mentioning saved it as. Run once, in the server's
// container:
//
//	docker exec ice9-teamgram /app/bin/usernames
//
// Each account is named the way a sign-up would name it and marked as given -
// a stranger cannot find it by that name - and its cached copy is dropped, so
// the next answer about it carries the name. It says what it gave to whom,
// and exits non-zero if anything failed or any username is held twice.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/teamgram/marmota/pkg/stores/sqlx"
	"github.com/teamgram/teamgram-server/app/service/biz/user/user"
	"github.com/teamgram/teamgram-server/pkg/usernames"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/kv"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type nameless struct {
	Id        int64  `db:"id"`
	FirstName string `db:"first_name"`
	LastName  string `db:"last_name"`
}

func main() {
	// What it gave to whom is the output; every query logged around it is not.
	logx.Disable()

	dsn := os.Getenv("USERNAMES_MYSQL_DSN")
	if dsn == "" {
		dsn = "teamgram:" + os.Getenv("MYSQL_PASSWORD") + "@tcp(mysql:3306)/teamgram?charset=utf8mb4&parseTime=true"
	}
	db := sqlx.NewMySQL(&sqlx.Config{DSN: dsn, Active: 2, Idle: 2})
	store := usernames.NewStore(db, func(name string) bool { return user.GetBotIdByName(name) > 0 })
	cache := kv.NewStore(kv.KvConf{{
		RedisConf: redis.RedisConf{
			Host: envOr("REDIS_HOST", "redis:6379"),
			Type: "node",
			Pass: os.Getenv("REDIS_PASS"),
		},
		Weight: 100,
	}})
	ctx := context.Background()

	var people []nameless
	if err := db.QueryRowsPartial(ctx, &people,
		"select id, first_name, last_name from users where deleted = 0 and username = '' and user_type = ? order by id",
		user.UserTypeRegular); err != nil && err != sqlx.ErrNotFound {
		fmt.Fprintf(os.Stderr, "cannot read the accounts without a username: %v\n", err)
		os.Exit(2)
	}

	failed := 0
	for _, person := range people {
		name, err := store.Assign(ctx, person.Id, person.FirstName, person.LastName)
		if err != nil {
			fmt.Printf("%d: no username: %v\n", person.Id, err)
			failed++
			continue
		}
		if _, err := cache.Del("user_data.2#" + strconv.FormatInt(person.Id, 10)); err != nil {
			fmt.Printf("%d -> %s, but its cached copy stays: %v\n", person.Id, name, err)
			failed++
			continue
		}
		fmt.Printf("%d -> %s\n", person.Id, name)
	}

	var twice []string
	if err := db.QueryRowsPartial(ctx, &twice,
		"select username from username group by username having count(*) > 1"); err != nil && err != sqlx.ErrNotFound {
		fmt.Fprintf(os.Stderr, "cannot check for usernames held twice: %v\n", err)
		os.Exit(2)
	}

	fmt.Printf("named %d of %d account(s) without a username; held twice: %d %v\n",
		len(people)-failed, len(people), len(twice), twice)
	if failed > 0 || len(twice) > 0 {
		os.Exit(1)
	}
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
