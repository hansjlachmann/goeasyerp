package salesposting_test

import (
	"errors"
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/salesposting"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// listening arms the test's subscribers (subscriptions last for the test binary)
var (
	listening  bool
	refuse     error
	postedInfo *salesposting.AfterPost
)

func init() {
	salesposting.OnBeforePost.Subscribe(func(e *salesposting.BeforePost) error {
		if listening {
			return refuse
		}
		return nil
	})
	salesposting.OnAfterPost.Subscribe(func(e *salesposting.AfterPost) error {
		if listening {
			postedInfo = e
		}
		return nil
	})
}

// A subscriber gets the posted document's numbers after the posting, and an error before it
// stops the posting
func TestPostingEvents(t *testing.T) {
	src := demoCompany(t)
	listening = true
	t.Cleanup(func() { listening, refuse, postedInfo = false, nil, nil })

	invoice := newDocument(t, src, gtables.SalesHeader_Document_type.Invoice, "C00020")
	addLine(t, invoice, 10000, gtables.SalesLine_Type.Item, "1000", "1")
	salesPost := post(t, invoice, false, false)
	if postedInfo == nil || postedInfo.InvoiceNo != salesPost.InvoiceNo || postedInfo.InvoiceNo == "" || postedInfo.SalesHeader.No != invoice.No {
		t.Errorf("OnAfterPost got %+v, want invoice %s", postedInfo, salesPost.InvoiceNo)
	}

	refuse = errors.New("refused by an app")
	postedInfo = nil
	order := newDocument(t, src, gtables.SalesHeader_Document_type.Invoice, "C00020")
	addLine(t, order, 10000, gtables.SalesLine_Type.Item, "1000", "1")
	order.Ship, order.Invoice = false, false
	err := salesposting.NewSalesPost(order).Run(order)
	if !errors.Is(err, refuse) || postedInfo != nil {
		t.Errorf("OnBeforePost's error: %v, after event %+v", err, postedInfo)
	}
}
