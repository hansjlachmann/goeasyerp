package salesposting

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/events"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Events of the sales posting (codeunit 80) that extension apps subscribe to (see
// foundation/events). Both run inside the posting's transaction: a subscriber's error stops
// the posting and nothing of it is saved.

// BeforePost is what OnBeforePost passes: the document about to be posted (its Ship and
// Invoice say what) — a subscriber may refuse it with an error
type BeforePost struct {
	SalesHeader *gtables.SalesHeader
}

// AfterPost is what OnAfterPost passes: the document posted and the numbers of the documents
// the posting wrote (blank when it wrote none); the document itself may be deleted already (an
// invoice, or an order invoiced completely) — its values are as posted
type AfterPost struct {
	SalesHeader *gtables.SalesHeader
	ShipmentNo  types.Code
	InvoiceNo   types.Code
	CrMemoNo    types.Code
}

var (
	// OnBeforePost: a sales document is about to be posted
	OnBeforePost = events.New[*BeforePost]("Sales-Post.OnBeforePost")
	// OnAfterPost: a sales document was posted
	OnAfterPost = events.New[*AfterPost]("Sales-Post.OnAfterPost")
)
