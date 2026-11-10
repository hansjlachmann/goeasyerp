package tables

import (
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// shippingAgentLogic is the business logic of table 291 Shipping_Agent.
// Add a trigger when it is needed: the available triggers are listed at the
// top of generated/tables/shippingagent_gen.go. The field rules of the definition
// (required, length, range) are checked after OnInsert and OnModify.
type shippingAgentLogic struct{}

func init() { gtables.RegisterShippingAgentLogic(shippingAgentLogic{}) }

// OnInsert runs before a new record is inserted
func (shippingAgentLogic) OnInsert(rec *gtables.ShippingAgent) error {
	return nil
}

// OnModify runs before a record is modified
func (shippingAgentLogic) OnModify(rec *gtables.ShippingAgent) error {
	return nil
}

// OnDelete runs before a record is deleted
func (shippingAgentLogic) OnDelete(rec *gtables.ShippingAgent) error {
	return nil
}

// OnRename runs before a primary key field is changed (rec.OldValue has the old value)
func (shippingAgentLogic) OnRename(rec *gtables.ShippingAgent) error {
	return nil
}

// GetTrackingInternetAddr is the Shipping Agent's GetTrackingInternetAddr: the Internet Address
// with the package tracking number in place of %1, with "https://" in front unless it has a
// scheme (classic: "http://" unless it starts with it)
func GetTrackingInternetAddr(shippingAgent *gtables.ShippingAgent, packageTrackingNo types.Text) string {
	httpStr := "https://"
	internetAddress := strings.TrimSpace(shippingAgent.Internet_address.String())
	if strings.HasPrefix(strings.ToLower(internetAddress), "http://") || strings.HasPrefix(strings.ToLower(internetAddress), "https://") {
		httpStr = ""
	}
	return httpStr + system.StrSubstNo(internetAddress, packageTrackingNo.String())
}
