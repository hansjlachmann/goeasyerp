package tables

import (
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
