package tables

import (
	"errors"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"strings"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/secrets"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
)

// maxPassword is the longest password whose encrypted form fits the 250-character column
const maxPassword = 154

func TestSMTPPasswordStoredEncrypted(t *testing.T) {
	defer secrets.SetKeyForTest("test key")()
	db := newTestDB(t)
	var setup gtables.SMTPSetup
	if err := setup.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	setup.InitWithDBType(db, "", database.DBTypeSQLite)
	setup.Enabled = true
	setup.Password = types.NewText("abcd efgh ijkl mnop")
	if !setup.Insert(true) {
		t.Fatalf("insert: %v", setup.TriggerError())
	}
	stored := func() string {
		var p string
		if err := db.QueryRow(`SELECT password FROM "SMTP_Setup"`).Scan(&p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	first := stored()
	if !secrets.IsEncrypted(first) || strings.Contains(first, "abcd") {
		t.Fatalf("stored password %q, want it encrypted", first)
	}

	// Changing another field keeps the stored password as it is (not encrypted twice)
	var again gtables.SMTPSetup
	again.InitWithDBType(db, "", database.DBTypeSQLite)
	if !again.Get("") {
		t.Fatal("get")
	}
	again.Smtp_server = types.NewText("smtp.example.com")
	if !again.Modify(true) {
		t.Fatalf("modify: %v", again.TriggerError())
	}
	if stored() != first {
		t.Error("password changed by modifying another field")
	}
	if plain, err := again.PlainPassword(); err != nil || plain != "abcd efgh ijkl mnop" {
		t.Errorf("PlainPassword = %q, %v", plain, err)
	}

	// The longest password still fits the column encrypted; a longer one is refused
	again.Password = types.NewText(strings.Repeat("x", maxPassword))
	if !again.Modify(true) {
		t.Fatalf("modify with %d characters: %v", maxPassword, again.TriggerError())
	}
	if n := len(stored()); n > 250 {
		t.Errorf("encrypted password is %d characters, column holds 250", n)
	}
	again.Password = types.NewText(strings.Repeat("x", maxPassword+1))
	if again.Modify(true) || again.TriggerError() == nil {
		t.Error("a password over the limit was accepted")
	}

	// With another key the password cannot be read
	if !again.Get("") {
		t.Fatal("get")
	}
	restore := secrets.SetKeyForTest("other key")
	if _, err := again.PlainPassword(); !errors.Is(err, secrets.ErrDecrypt) {
		t.Errorf("PlainPassword with another key: %v, want ErrDecrypt", err)
	}
	restore()
}

func TestEncryptStoredSecrets(t *testing.T) {
	db := newTestDB(t)
	var setup gtables.SMTPSetup
	if err := setup.CreateTableWithDBType(db, "", database.DBTypeSQLite); err != nil {
		t.Fatal(err)
	}
	// Saved before encryption (no key)
	restore := secrets.SetKeyForTest("")
	setup.InitWithDBType(db, "", database.DBTypeSQLite)
	setup.Password = types.NewText("old password")
	if !setup.Insert(true) {
		t.Fatal("insert")
	}
	if n, err := gtables.EncryptStoredSecrets(db, database.DBTypeSQLite, nil); n != 0 || err != nil {
		t.Errorf("without a key: %d, %v; want nothing done", n, err)
	}
	restore()

	defer secrets.SetKeyForTest("test key")()
	if n, err := gtables.EncryptStoredSecrets(db, database.DBTypeSQLite, nil); n != 1 || err != nil {
		t.Fatalf("encrypted %d, %v; want the password encrypted", n, err)
	}
	var check gtables.SMTPSetup
	check.InitWithDBType(db, "", database.DBTypeSQLite)
	if !check.Get("") || !secrets.IsEncrypted(check.Password.String()) {
		t.Fatalf("stored %q, want encrypted", check.Password)
	}
	if plain, err := check.PlainPassword(); err != nil || plain != "old password" {
		t.Errorf("PlainPassword = %q, %v", plain, err)
	}
	if n, _ := gtables.EncryptStoredSecrets(db, database.DBTypeSQLite, nil); n != 0 {
		t.Error("an encrypted password was encrypted again")
	}
}
