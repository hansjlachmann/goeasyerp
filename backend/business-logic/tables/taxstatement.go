package tables

// Tax statements: table 255 Tax Statement Template, 257 Tax Statement Name and 256 Tax Statement
// Line. A template has statements, a statement its lines; a line totals G/L accounts (Account
// Totaling, a filter), tax entries (Tax Entry Totaling: the type, posting groups, jurisdiction
// and use tax, and which amount) or other rows (Row Totaling, a filter on Row No.), or is a text.
// The amounts are calculated by `business-logic/taxstatement` (report 12, the preview). The
// tables' triggers are in taxstatementtemplate.go, taxstatementname.go and taxstatementline.go.

// TaxStatementPageID is page 317 Tax Statement, the page of a template without one
const TaxStatementPageID = 317

// TaxStatementReportID is report 12 Tax Statement
const TaxStatementReportID = 12
