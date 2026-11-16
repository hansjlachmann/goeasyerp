package docmail

import (
	"bytes"
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	_ "github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/tables"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/mail"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// fakeMailer keeps what it was asked to send
type fakeMailer struct {
	enabled     bool
	to          []string
	subject     string
	body        string
	attachments []mail.Attachment
}

func (m *fakeMailer) Enabled() bool { return m.enabled }
func (m *fakeMailer) Send(to []string, subject, body string) error {
	return m.SendWithAttachments(to, subject, body, nil)
}
func (m *fakeMailer) SendWithAttachments(to []string, subject, body string, attachments []mail.Attachment) error {
	m.to, m.subject, m.body, m.attachments = to, subject, body, attachments
	return nil
}

func mustOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// emailCompany: a company with SMTP Setup, customer C1 (Norwegian, with an e-mail address) and
// sales quote SQ1 with a text line
func emailCompany(t *testing.T) (ftables.Source, *gtables.SalesHeader) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	mustOK(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	mustOK(t, tables.CreateCompanyTables(db, "mail", database.DBTypeSQLite))
	mustOK(t, (&gtables.SMTPSetup{}).CreateTableWithDBType(db, "", database.DBTypeSQLite))
	src := ftables.DBSource(db, "mail", database.DBTypeSQLite)
	fcodeunits.SetCurrentContext("HANS", "Hans", "mail", "en-US")
	t.Cleanup(fcodeunits.ClearCurrentContext)

	var setup gtables.SMTPSetup
	setup.InitWithDBType(db, "", database.DBTypeSQLite)
	setup.Enabled = true
	setup.Smtp_server = "smtp.example.com"
	setup.From_address = "sales@example.com"
	mustOK(t, setup.InsertOrError(false))

	companyInfo := gtables.GetCompanyInformation(src)
	companyInfo.Name = "Nordkapp Kontor AS"
	mustOK(t, companyInfo.InsertOrError(false))
	cust := gtables.NewCustomer(src)
	cust.No = "C1"
	cust.Name = "Fjord Møbler AS"
	cust.E_mail = "innkjop@fjord.example.com"
	cust.Language_code = "NB-NO"
	cust.Customer_posting_group = "DOM"
	cust.Gen_bus_posting_group = "DOM"
	mustOK(t, cust.InsertOrError(false))
	salesHeader := gtables.NewSalesHeader(src)
	salesHeader.Document_type = gtables.SalesHeader_Document_type.Quote
	salesHeader.No = "SQ1"
	mustOK(t, salesHeader.InsertOrError(true))
	mustOK(t, salesHeader.Validate("sell_to_customer_no", types.Code("C1")))
	mustOK(t, salesHeader.ModifyOrError(true))
	salesLine := gtables.NewSalesLine(src)
	salesLine.Document_type = gtables.SalesLine_Document_type.Quote
	salesLine.Document_no = "SQ1"
	salesLine.Line_no = 10000
	salesLine.Description = "Delivered in October"
	mustOK(t, salesLine.InsertOrError(true))
	return src, salesHeader
}

// withDialog runs fn with a dialog whose e-mail form is answered by answer (the form's fields
// with their defaults in; nil: Cancel)
func withDialog(t *testing.T, answer func(defaults map[string]string) map[string]string, fn func()) {
	t.Helper()
	d := fcodeunits.OpenDialogWithID("", "docmail-test-"+t.Name())
	fcodeunits.SetCurrentDialog(d)
	defer fcodeunits.ClearCurrentDialog()
	go func() {
		for ev := range d.Events() {
			if ev.EventType != "request_input" {
				continue
			}
			defaults := map[string]string{}
			fields, _ := ev.Data["fields"].([]fcodeunits.InputField)
			for _, f := range fields {
				defaults[f.Name] = f.Default
			}
			for !d.RespondToInput(ev.ConfirmID, answer(defaults)) {
				time.Sleep(time.Millisecond)
			}
		}
	}()
	fn()
}

func TestEmailSalesQuote(t *testing.T) {
	_, salesHeader := emailCompany(t)
	mailer := &fakeMailer{enabled: true}
	newMailer = func(mail.Config) mail.AttachmentSender { return mailer }
	t.Cleanup(func() { newMailer = func(cfg mail.Config) mail.AttachmentSender { return mail.NewSMTPMailer(cfg) } })

	// The form starts with the customer's address and a subject and message in the quote's
	// language (the customer's: Norwegian); the printout is attached
	var defaults map[string]string
	var result fcodeunits.Result
	var err error
	withDialog(t, func(d map[string]string) map[string]string { defaults = d; return d }, func() {
		result, err = EmailSalesHeader(salesHeader)
	})
	mustOK(t, err)
	if defaults["to"] != "innkjop@fjord.example.com" || defaults["subject"] != "Salgstilbud SQ1 fra Nordkapp Kontor AS" ||
		!strings.Contains(defaults["body"], "Vedlagt følger salgstilbud SQ1.") || !strings.HasSuffix(defaults["body"], "Nordkapp Kontor AS") {
		t.Errorf("form defaults %q", defaults)
	}
	if strings.Join(mailer.to, ",") != "innkjop@fjord.example.com" || mailer.subject != defaults["subject"] || len(mailer.attachments) != 1 {
		t.Fatalf("sent to %v, subject %q, %d attachments", mailer.to, mailer.subject, len(mailer.attachments))
	}
	attachment := mailer.attachments[0]
	if !bytes.HasPrefix(attachment.Data, []byte("%PDF-")) || attachment.ContentType != "application/pdf" || !strings.HasSuffix(attachment.Name, "SQ1.pdf") {
		t.Errorf("attachment %q %q", attachment.Name, attachment.ContentType)
	}
	if result.Message != "The e-mail was sent to innkjop@fjord.example.com." {
		t.Errorf("message %q", result.Message)
	}

	// In the user's language when the document has none; the name is in lower case in the sentence
	salesHeader.Language_code = ""
	withDialog(t, func(d map[string]string) map[string]string { defaults = d; return nil }, func() {
		_, err = EmailSalesHeader(salesHeader)
	})
	mustOK(t, err)
	if defaults["subject"] != "Sales Quote SQ1 from Nordkapp Kontor AS" || !strings.Contains(defaults["body"], "Please find our sales quote SQ1 attached.") {
		t.Errorf("form defaults in English %q", defaults)
	}

	// Cancel sends nothing
	mailer.to = nil
	withDialog(t, func(map[string]string) map[string]string { return nil }, func() {
		_, err = EmailSalesHeader(salesHeader)
	})
	mustOK(t, err)
	if mailer.to != nil {
		t.Error("sent after Cancel")
	}

	// An address that is not one is refused
	withDialog(t, func(d map[string]string) map[string]string { d["to"] = "innkjop@"; return d }, func() {
		_, err = EmailSalesHeader(salesHeader)
	})
	if err == nil || !strings.Contains(err.Error(), "is not a valid e-mail address") {
		t.Errorf("bad address: %v", err)
	}

	// Without SMTP Setup nothing is asked or sent
	mailer.enabled = false
	_, err = EmailSalesHeader(salesHeader)
	if err == nil || !strings.Contains(err.Error(), "E-mail is not set up") {
		t.Errorf("no SMTP: %v", err)
	}
}
