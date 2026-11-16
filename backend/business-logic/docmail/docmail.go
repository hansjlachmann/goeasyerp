// Package docmail is routine 908 Document E-mail: a document is sent by e-mail with the
// printouts of its usage in Report Selections attached as PDF files. The user confirms the
// recipients, the subject and the message, which are filled in from the customer or vendor and
// written in the document's language; the e-mail goes out through the SMTP Setup.
package docmail

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/jobqueue"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/functions"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/mail"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// DocumentEmailID is routine 908 Document E-mail
const DocumentEmailID = 908

// newMailer builds the mailer of the SMTP Setup (tests send through a fake)
var newMailer = func(cfg mail.Config) mail.AttachmentSender { return mail.NewSMTPMailer(cfg) }

func init() {
	email := func(table string, fn functions.Func) {
		functions.Register("table", table, "EmailRecords", fn, functions.WithPrint())
	}
	email(gtables.SalesHeaderTableName, functions.For(EmailSalesHeader))
	email(gtables.PurchaseHeaderTableName, functions.For(EmailPurchHeader))
	email(gtables.SalesShipmentHeaderTableName, functions.For(func(rec *gtables.SalesShipmentHeader) (fcodeunits.Result, error) {
		return send(document{rec: rec, usage: gtables.ReportSelections_Usage.SShipment, name: "EMAIL_DOC_SALES_SHIPMENT",
			no: rec.No, languageCode: rec.Language_code, customerNo: rec.Sell_to_customer_no})
	}))
	email(gtables.SalesInvoiceHeaderTableName, functions.For(func(rec *gtables.SalesInvoiceHeader) (fcodeunits.Result, error) {
		return send(document{rec: rec, usage: gtables.ReportSelections_Usage.SInvoice, name: "EMAIL_DOC_SALES_INVOICE",
			no: rec.No, languageCode: rec.Language_code, customerNo: rec.Bill_to_customer_no})
	}))
	email(gtables.SalesCrMemoHeaderTableName, functions.For(func(rec *gtables.SalesCrMemoHeader) (fcodeunits.Result, error) {
		return send(document{rec: rec, usage: gtables.ReportSelections_Usage.SCrMemo, name: "EMAIL_DOC_SALES_CR_MEMO",
			no: rec.No, languageCode: rec.Language_code, customerNo: rec.Bill_to_customer_no})
	}))
	email(gtables.IssuedReminderHeaderTableName, functions.For(func(rec *gtables.IssuedReminderHeader) (fcodeunits.Result, error) {
		return send(document{rec: rec, usage: gtables.ReportSelections_Usage.Reminder, name: "EMAIL_DOC_REMINDER",
			no: rec.No, languageCode: rec.Language_code, customerNo: rec.Customer_no})
	}))
	email(gtables.IssuedFinChargeMemoHeaderTableName, functions.For(func(rec *gtables.IssuedFinChargeMemoHeader) (fcodeunits.Result, error) {
		return send(document{rec: rec, usage: gtables.ReportSelections_Usage.FinCharge, name: "EMAIL_DOC_FIN_CHARGE_MEMO",
			no: rec.No, languageCode: rec.Language_code, customerNo: rec.Customer_no})
	}))
}

// document is what is sent: the record the reports print, the usage of its reports, the text
// key of its name, its number and language, and the customer or vendor it goes to
type document struct {
	rec          ftables.Source
	usage        gtables.ReportSelectionsUsage
	name         string
	no           types.Code
	languageCode types.Code
	customerNo   types.Code
	vendorNo     types.Code
}

// EmailSalesHeader sends a sales quote, order (its confirmation) or blanket order to the
// sell-to customer; other document types have nothing to send
func EmailSalesHeader(salesHeader *gtables.SalesHeader) (fcodeunits.Result, error) {
	d := document{rec: salesHeader, no: salesHeader.No, languageCode: salesHeader.Language_code, customerNo: salesHeader.Sell_to_customer_no}
	switch salesHeader.Document_type {
	case gtables.SalesHeader_Document_type.Quote:
		d.usage, d.name = gtables.ReportSelections_Usage.SQuote, "EMAIL_DOC_SALES_QUOTE"
	case gtables.SalesHeader_Document_type.Order:
		d.usage, d.name = gtables.ReportSelections_Usage.SOrder, "EMAIL_DOC_ORDER_CONFIRMATION"
	case gtables.SalesHeader_Document_type.BlanketOrder:
		d.usage, d.name = gtables.ReportSelections_Usage.SBlanket, "EMAIL_DOC_BLANKET_SALES_ORDER"
	default:
		return fcodeunits.Result{Success: true}, nil
	}
	return send(d)
}

// EmailPurchHeader sends a purchase quote (a request for a quote), order or blanket order to the
// buy-from vendor; other document types have nothing to send
func EmailPurchHeader(purchHeader *gtables.PurchaseHeader) (fcodeunits.Result, error) {
	d := document{rec: purchHeader, no: purchHeader.No, languageCode: purchHeader.Language_code, vendorNo: purchHeader.Buy_from_vendor_no}
	switch purchHeader.Document_type {
	case gtables.PurchaseHeader_Document_type.Quote:
		d.usage, d.name = gtables.ReportSelections_Usage.PQuote, "EMAIL_DOC_PURCHASE_QUOTE"
	case gtables.PurchaseHeader_Document_type.Order:
		d.usage, d.name = gtables.ReportSelections_Usage.POrder, "EMAIL_DOC_PURCHASE_ORDER"
	case gtables.PurchaseHeader_Document_type.BlanketOrder:
		d.usage, d.name = gtables.ReportSelections_Usage.PBlanket, "EMAIL_DOC_BLANKET_PURCHASE_ORDER"
	default:
		return fcodeunits.Result{Success: true}, nil
	}
	return send(d)
}

// send asks for the recipients, subject and message, prints the document's reports as PDF files
// and sends them. Cancelling the dialog sends nothing.
func send(d document) (fcodeunits.Result, error) {
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()
	db, dbType := d.rec.GetDB(), d.rec.GetDBType()

	cfg, err := jobqueue.LoadSMTPConfig(db, dbType)
	if err != nil {
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("JOBQUEUE_TEST_MAIL_BAD_PASSWORD", lang,
			ts.FieldCaption(gtables.SMTPSetupTableName, "password", lang), ts.TableCaption(gtables.SMTPSetupTableName, lang)))
	}
	mailer := newMailer(cfg)
	if !mailer.Enabled() {
		return fcodeunits.Result{}, errors.New(ts.Message("EMAIL_NOT_SET_UP", lang))
	}

	// the subject and message in the document's language
	docLang := documentLanguage(d.languageCode, lang)
	companyName := companyName(d.rec)
	docName := ts.Message(d.name, docLang)
	subject := ts.MessageWithParams("EMAIL_SUBJECT_DEFAULT", docLang, docName, d.no.String(), companyName)
	body := ts.MessageWithParams("EMAIL_BODY_DEFAULT", docLang, strings.ToLower(docName), d.no.String(), companyName)
	values := fcodeunits.RequestInput(ts.Message("EMAIL_DIALOG_TITLE", lang), []fcodeunits.InputField{
		{Name: "to", Label: ts.Message("EMAIL_TO", lang), Type: "text", Required: true, Default: recipient(d)},
		{Name: "subject", Label: ts.Message("EMAIL_SUBJECT", lang), Type: "text", Required: true, Default: subject},
		{Name: "body", Label: ts.Message("EMAIL_MESSAGE", lang), Type: "multiline", Default: body},
	})
	if values == nil {
		return fcodeunits.Result{Success: true}, nil
	}
	to, err := mail.ParseAddressList(values["to"])
	if err != nil {
		var listErr *mail.AddressListError
		if errors.As(err, &listErr) {
			return fcodeunits.Result{}, errors.New(ts.MessageWithParams("EMAIL_INVALID_ADDRESS", lang, listErr.Address))
		}
		return fcodeunits.Result{}, err
	}
	if len(to) == 0 {
		return fcodeunits.Result{}, errors.New(ts.Message("EMAIL_NO_RECIPIENT", lang))
	}

	printouts, err := reports.PrintDocuments(d.rec, d.usage, false)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	var attachments []mail.Attachment
	for _, printout := range printouts {
		if printout.PDF == nil {
			continue
		}
		attachments = append(attachments, mail.Attachment{Name: pdfName(printout.FileName, d.no), ContentType: "application/pdf", Data: printout.PDF})
	}
	if len(attachments) == 0 {
		return fcodeunits.Result{}, errors.New(ts.Message("EMAIL_NOTHING_TO_SEND", lang))
	}

	err = mailer.SendWithAttachments(to, values["subject"], values["body"], attachments)
	var rcptErr *mail.RecipientsError
	switch {
	case err == nil:
		text := ts.MessageWithParams("EMAIL_SENT", lang, mail.JoinAddressList(to))
		return fcodeunits.Result{Success: true, Message: text, Dialog: &fcodeunits.DialogResult{Message: text, Type: "success"}}, nil
	case errors.As(err, &rcptErr) && rcptErr.Sent > 0:
		rejected := make(map[string]bool, len(rcptErr.Rejected))
		var notSent []string
		for _, r := range rcptErr.Rejected {
			rejected[r.Recipient] = true
			notSent = append(notSent, fmt.Sprintf("%s (%v)", r.Recipient, r.Err))
		}
		var sent []string
		for _, r := range to {
			if !rejected[r] {
				sent = append(sent, r)
			}
		}
		text := ts.MessageWithParams("EMAIL_SENT_PARTIAL", lang, mail.JoinAddressList(sent), strings.Join(notSent, "; "))
		return fcodeunits.Result{Success: true, Message: text, Dialog: &fcodeunits.DialogResult{Message: text, Type: "warning"}}, nil
	default:
		return fcodeunits.Result{}, errors.New(ts.MessageWithParams("EMAIL_SEND_FAILED", lang, err.Error()))
	}
}

// recipient: the e-mail address of the document's customer or vendor ("" when it has none)
func recipient(d document) string {
	if d.customerNo != "" {
		cust := gtables.NewCustomer(d.rec)
		if cust.Get(d.customerNo) {
			return cust.E_mail.String()
		}
	}
	if d.vendorNo != "" {
		vend := gtables.NewVendor(d.rec)
		if vend.Get(d.vendorNo) {
			return vend.E_mail.String()
		}
	}
	return ""
}

// documentLanguage: the application language of a Language Code (NB-NO → nb-NO), else the user's
func documentLanguage(languageCode types.Code, userLanguage string) string {
	for _, language := range i18n.GetInstance().GetSupportedLanguages() {
		if strings.EqualFold(language, languageCode.String()) {
			return language
		}
	}
	return userLanguage
}

// companyName: the name on the documents (Company Information), else the company's display name
func companyName(src ftables.Source) string {
	companyInfo := gtables.GetCompanyInformation(src)
	if companyInfo.Name != "" {
		return companyInfo.Name.String()
	}
	return jobqueue.CompanyName(src.GetDB(), src.GetDBType(), src.GetCompany())
}

// pdfName: the printout's file name, ending in .pdf (the document number when the report has none)
func pdfName(fileName string, no types.Code) string {
	if fileName == "" {
		fileName = no.String()
	}
	if !strings.HasSuffix(strings.ToLower(fileName), ".pdf") {
		fileName += ".pdf"
	}
	return fileName
}
