package database

import (
	"database/sql"
	"testing"
)

func TestMaxConnsFromEnv(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int
	}{
		{"", DefaultMaxConns},
		{"60", 60},
		{"2", 2},
		{"1", DefaultMaxConns}, // one connection would hang a transaction that reads beside it
		{"0", DefaultMaxConns},
		{"-5", DefaultMaxConns},
		{"lots", DefaultMaxConns},
	} {
		t.Setenv("DB_MAX_CONNS", tc.value)
		got := MaxConnsFromEnv()
		if got != tc.want {
			t.Errorf("DB_MAX_CONNS=%q: %d, want %d", tc.value, got, tc.want)
		}
	}
}

func TestLimitPool(t *testing.T) {
	t.Setenv("DB_MAX_CONNS", "5")
	conn, err := sql.Open(SQLiteDriver, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	limitPool(conn)
	if got := conn.Stats().MaxOpenConnections; got != 5 {
		t.Errorf("max open connections %d, want 5", got)
	}
}
