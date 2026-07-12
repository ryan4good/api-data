package system

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestMySQLRepositoryAgainstDevelopmentSeed(t *testing.T) {
	dsn := os.Getenv("MYSQL_INTEGRATION_DSN")
	if dsn == "" {
		t.Skip("set MYSQL_INTEGRATION_DSN to run the MySQL/MariaDB compatibility test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repository, err := OpenMySQLRepository(ctx, dsn, MySQLPoolConfig{
		MaxOpenConns: 2, MaxIdleConns: 1, ConnMaxLifetime: time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	const (
		aliceID = "10000000-0000-4000-8000-000000000001"
		omsID   = "20000000-0000-4000-8000-000000000001"
		wmsID   = "20000000-0000-4000-8000-000000000002"
	)
	items, err := repository.ListAuthorized(ctx, aliceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].System.ID != omsID {
		t.Fatalf("Alice scope = %#v, want OMS only", items)
	}
	if _, found, err := repository.GetAuthorized(ctx, wmsID, aliceID); err != nil || found {
		t.Fatalf("Alice must not access WMS: found=%v err=%v", found, err)
	}
	members, err := repository.ListMembers(ctx, omsID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) == 0 {
		t.Fatal("OMS development seed has no members")
	}
}
