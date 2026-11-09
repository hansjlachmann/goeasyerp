package errors

import "strconv"

// Resources (table 156 and the tables around it)
const (
	ErrResCodeWhenTypeAll ErrorCode = "ERR_RES_CODE_WHEN_TYPE_ALL"
)

// ResCodeWhenTypeAll: a price or cost for all resources has no code ("Code cannot be specified
// when Type is All")
func ResCodeWhenTypeAll(table, field, field2 string, value any) *AppError {
	return &AppError{Code: ErrResCodeWhenTypeAll, Table: table, Field: field, Field2: field2, Value: value}
}

// Bills of materials (BOM journal posting)
const (
	ErrBOMCostingMethod      ErrorCode = "ERR_BOM_COSTING_METHOD"
	ErrBOMCompCostingMethod  ErrorCode = "ERR_BOM_COMP_COSTING_METHOD"
	ErrBOMComponentOfItself  ErrorCode = "ERR_BOM_COMPONENT_OF_ITSELF"
	ErrBOMNotBillOfMaterials ErrorCode = "ERR_BOM_NOT_BILL_OF_MATERIALS"
	ErrBOMNoSpaceToExplode   ErrorCode = "ERR_BOM_NO_SPACE_TO_EXPLODE"
	ErrBOMTooManyLevels      ErrorCode = "ERR_BOM_TOO_MANY_LEVELS"
)

// BOMCostingMethod: an assembled item's costing method cannot be Specific ("The Costing Method
// for BOM 1000 cannot be Specific.")
func BOMCostingMethod(table, field, no string, value any) *AppError {
	return &AppError{Code: ErrBOMCostingMethod, Table: table, Field: field, Params: []string{no}, Value: value}
}

// BOMCompCostingMethod: a component's costing method cannot be Specific
func BOMCompCostingMethod(table, field, no string, value any) *AppError {
	return &AppError{Code: ErrBOMCompCostingMethod, Table: table, Field: field, Params: []string{no}, Value: value}
}

// BOMComponentOfItself: a bill of materials cannot be a component of itself
func BOMComponentOfItself() *AppError { return &AppError{Code: ErrBOMComponentOfItself} }

// BOMNotBillOfMaterials: an item (no.) to explode is not a bill of materials
func BOMNotBillOfMaterials(no string) *AppError {
	return &AppError{Code: ErrBOMNotBillOfMaterials, Params: []string{no}}
}

// BOMNoSpaceToExplode: there are no free line numbers for the exploded lines
func BOMNoSpaceToExplode() *AppError { return &AppError{Code: ErrBOMNoSpaceToExplode} }

// BOMTooManyLevels: a bill of materials has more levels than the cost calculation follows
func BOMTooManyLevels(levels int) *AppError {
	return &AppError{Code: ErrBOMTooManyLevels, Params: []string{strconv.Itoa(levels)}}
}

// Extended texts (tables 279/280, codeunit 378)
const (
	ErrExtTextNoResourcePurch ErrorCode = "ERR_EXT_TEXT_NO_RESOURCE_PURCH"
	ErrExtTextNoSpace         ErrorCode = "ERR_EXT_TEXT_NO_SPACE"
)

// ExtTextNoResourcePurch: an extended text of a resource cannot be used on purchases ("You cannot
// purchase resources.")
func ExtTextNoResourcePurch(table, field string) *AppError {
	return &AppError{Code: ErrExtTextNoResourcePurch, Table: table, Field: field}
}

// ExtTextNoSpace: there are no free line numbers for the extended text lines
func ExtTextNoSpace() *AppError { return &AppError{Code: ErrExtTextNoSpace} }
