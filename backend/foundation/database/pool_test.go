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

func TestPasswordRefusal(t *testing.T) {
	for _, tc := range []struct {
		password string
		warns    bool
	}{
		{"", true}, // the default openerp
		{"openerp", true},
		{" OpenERP ", true},
		{"postgres", true},
		{"9f2c41d0a7b3e5f8c6d1a2b4", false},
	} {
		got := passwordRefusal(tc.password) != ""
		if got != tc.warns {
			t.Errorf("password %q: refused %v, want %v", tc.password, got, tc.warns)
		}
	}

	// only a server with a database (DB_HOST), and not a development database
	t.Setenv("DB_PASSWORD", "openerp")
	t.Setenv("DB_HOST", "")
	if PasswordRefusal() != "" {
		t.Error("refused without DB_HOST (SQLite)")
	}
	t.Setenv("DB_HOST", "db")
	t.Setenv(EnvKeepSuperuser, "true")
	if PasswordRefusal() != "" {
		t.Error("refused on a development database (DB_KEEP_SUPERUSER)")
	}
	t.Setenv(EnvKeepSuperuser, "")
	if PasswordRefusal() == "" {
		t.Error("openerp accepted on a server")
	}
}
