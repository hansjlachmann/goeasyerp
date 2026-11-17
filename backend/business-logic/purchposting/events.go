package purchposting

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/events"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Events of the purchase posting (codeunit 90) that extension apps subscribe to (see
// foundation/events). Both run inside the posting's transaction: a subscriber's error stops
// the posting and nothing of it is saved.

// BeforePost is what OnBeforePost passes: the document about to be posted (its Receive and
// Invoice say what) — a subscriber may refuse it with an error
type BeforePost struct {
	PurchHeader *gtables.PurchaseHeader
}

// AfterPost is what OnAfterPost passes: the document posted and the numbers of the documents
// the posting wrote (blank when it wrote none); the document itself may be deleted already
type AfterPost struct {
	PurchHeader *gtables.PurchaseHeader
	ReceiptNo   types.Code
	InvoiceNo   types.Code
	CrMemoNo    types.Code
}

var (
	// OnBeforePost: a purchase document is about to be posted
	OnBeforePost = events.New[*BeforePost]("Purch.-Post.OnBeforePost")
	// OnAfterPost: a purchase document was posted
	OnAfterPost = events.New[*AfterPost]("Purch.-Post.OnAfterPost")
)
