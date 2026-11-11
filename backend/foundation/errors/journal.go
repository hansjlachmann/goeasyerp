package errors

import "fmt"

// Journal and posting errors (general journal lines, templates, batches, posting codeunits)
const (
	ErrJnlAccountTypes        ErrorCode = "ERR_JNL_ACCOUNT_TYPES"
	ErrJnlOptionNotAllowed    ErrorCode = "ERR_JNL_OPTION_NOT_ALLOWED"
	ErrJnlOptionInternal      ErrorCode = "ERR_JNL_OPTION_INTERNAL"
	ErrJnlBankAccountRequired ErrorCode = "ERR_JNL_BANK_ACCOUNT_REQUIRED"
	ErrJnlWithoutField        ErrorCode = "ERR_JNL_WITHOUT_FIELD"
	ErrJnlRecurringOnlyField  ErrorCode = "ERR_JNL_RECURRING_ONLY_FIELD"
	ErrJnlMustNotBe           ErrorCode = "ERR_JNL_MUST_NOT_BE"
	ErrJnlTemplateTypeNoPage  ErrorCode = "ERR_JNL_TEMPLATE_TYPE_NO_PAGE"
)

// JnlAccountTypes: Account Type or Bal. Account Type must be G/L Account or Bank Account (a
// line cannot post customer/vendor to customer/vendor)
func JnlAccountTypes(table, field, field2 string) *AppError {
	return &AppError{Code: ErrJnlAccountTypes, Table: table, Field: field, Field2: field2}
}

// JnlOptionNotAllowed: a field cannot have an option value ("Bal. Account Type cannot be
// Employee.")
func JnlOptionNotAllowed(table, field string, value any) *AppError {
	return &AppError{Code: ErrJnlOptionNotAllowed, Table: table, Field: field, Value: value}
}

// JnlOptionInternal: an option value only the system sets (Gen. Posting Type Settlement)
func JnlOptionInternal(table, field string, value any) *AppError {
	return &AppError{Code: ErrJnlOptionInternal, Table: table, Field: field, Value: value}
}

// JnlBankAccountRequired: Account Type or Bal. Account Type must be a bank account
func JnlBankAccountRequired(table, field, field2 string) *AppError {
	return &AppError{Code: ErrJnlBankAccountRequired, Table: table, Field: field, Field2: field2}
}

// JnlWithoutField: a field cannot be filled in without another (Currency Factor without
// Currency Code)
func JnlWithoutField(table, field, field2 string) *AppError {
	return &AppError{Code: ErrJnlWithoutField, Table: table, Field: field, Field2: field2}
}

// JnlRecurringOnlyField: on recurring journals only one numbering field can be filled in
func JnlRecurringOnlyField(table, field string) *AppError {
	return &AppError{Code: ErrJnlRecurringOnlyField, Table: table, Field: field}
}

// JnlMustNotBe: a field must not have a value (Posting No. Series equal to No. Series)
func JnlMustNotBe(table, field, value string) *AppError {
	return &AppError{Code: ErrJnlMustNotBe, Table: table, Field: field, Params: []string{value}}
}

// JnlTemplateTypeNoPage: a journal template type without a journal page yet (Assets, Payroll
// Accrual)
func JnlTemplateTypeNoPage(table, field string, value any) *AppError {
	return &AppError{Code: ErrJnlTemplateTypeNoPage, Table: table, Field: field, Value: value}
}

// Checking and posting journal lines (Gen. Jnl.-Check Line, Post Line, Post Batch)
const (
	ErrJnlClosingDateGLOnly    ErrorCode = "ERR_JNL_CLOSING_DATE_GL_ONLY"
	ErrJnlDateNotAllowed       ErrorCode = "ERR_JNL_DATE_NOT_ALLOWED"
	ErrJnlSameSign             ErrorCode = "ERR_JNL_SAME_SIGN"
	ErrJnlDifferentSign        ErrorCode = "ERR_JNL_DIFFERENT_SIGN"
	ErrJnlMustBePositive       ErrorCode = "ERR_JNL_MUST_BE_POSITIVE"
	ErrJnlMustBeNegative       ErrorCode = "ERR_JNL_MUST_BE_NEGATIVE"
	ErrJnlTaxSum               ErrorCode = "ERR_JNL_TAX_SUM"
	ErrJnlNeedsRounding        ErrorCode = "ERR_JNL_NEEDS_ROUNDING"
	ErrJnlPostingCodeMandatory ErrorCode = "ERR_JNL_POSTING_CODE_MANDATORY"
	ErrJnlPostingCodeSame      ErrorCode = "ERR_JNL_POSTING_CODE_SAME"
	ErrJnlPostingCodeBlank     ErrorCode = "ERR_JNL_POSTING_CODE_BLANK"
	ErrJnlBankPmtCurrency      ErrorCode = "ERR_JNL_BANK_PMT_CURRENCY"
	ErrJnlRequiredWhen         ErrorCode = "ERR_JNL_REQUIRED_WHEN"
	ErrJnlCheckExists          ErrorCode = "ERR_JNL_CHECK_EXISTS"
	ErrJnlCheckPrintingNotYet  ErrorCode = "ERR_JNL_CHECK_PRINTING_NOT_YET"
	ErrCheckAlreadyExists      ErrorCode = "ERR_CHECK_ALREADY_EXISTS"
	ErrCheckAmountTooLong      ErrorCode = "ERR_CHECK_AMOUNT_TOO_LONG"
	ErrJnlNameTooLong          ErrorCode = "ERR_JNL_NAME_TOO_LONG"
	ErrJnlCorrectionSame       ErrorCode = "ERR_JNL_CORRECTION_SAME"
	ErrJnlDocOutOfBalance      ErrorCode = "ERR_JNL_DOC_OUT_OF_BALANCE"
	ErrJnlAddCurrOutOfBalance  ErrorCode = "ERR_JNL_ADD_CURR_OUT_OF_BALANCE"
	ErrJnlDateOutOfBalance     ErrorCode = "ERR_JNL_DATE_OUT_OF_BALANCE"
	ErrJnlDocOutOfBalanceCurr  ErrorCode = "ERR_JNL_DOC_OUT_OF_BALANCE_CURRENCY"
	ErrJnlDateOutOfBalanceCurr ErrorCode = "ERR_JNL_DATE_OUT_OF_BALANCE_CURRENCY"
	ErrJnlTaxMultipleCustVend  ErrorCode = "ERR_JNL_TAX_MULTIPLE_CUST_VEND"
	ErrJnlMaxPostingNoSeries   ErrorCode = "ERR_JNL_MAX_POSTING_NO_SERIES"
)

// JnlClosingDateGLOnly: a closing date is only allowed for G/L entries
func JnlClosingDateGLOnly(table, field string) *AppError {
	return &AppError{Code: ErrJnlClosingDateGLOnly, Table: table, Field: field}
}

// JnlDateNotAllowed: the posting date is outside the user's allowed posting dates
func JnlDateNotAllowed(table, field string) *AppError {
	return &AppError{Code: ErrJnlDateNotAllowed, Table: table, Field: field}
}

// JnlSameSign: a field must have the same sign as another (Amount (LCY) and Amount)
func JnlSameSign(table, field, field2 string) *AppError {
	return &AppError{Code: ErrJnlSameSign, Table: table, Field: field, Field2: field2}
}

// JnlDifferentSign: a field must have a different sign than another
func JnlDifferentSign(table, field, field2 string) *AppError {
	return &AppError{Code: ErrJnlDifferentSign, Table: table, Field: field, Field2: field2}
}

// JnlMustBePositive / JnlMustBeNegative: the sign an amount must have for its document type
func JnlMustBePositive(table, field string) *AppError {
	return &AppError{Code: ErrJnlMustBePositive, Table: table, Field: field}
}

func JnlMustBeNegative(table, field string) *AppError {
	return &AppError{Code: ErrJnlMustBeNegative, Table: table, Field: field}
}

// JnlTaxSum: Tax Amount + Tax Base Amount must be the amount
func JnlTaxSum(table, field, field2 string) *AppError {
	return &AppError{Code: ErrJnlTaxSum, Table: table, Field: field, Field2: field2}
}

// JnlNeedsRounding: an amount is not rounded to the currency's precision
func JnlNeedsRounding(table, field, value string) *AppError {
	return &AppError{Code: ErrJnlNeedsRounding, Table: table, Field: field, Params: []string{value}}
}

// JnlPostingCodeMandatory / Same / Blank: the G/L account's Dimension 1/2 Posting rule
func JnlPostingCodeMandatory(table, field, glAccNo string) *AppError {
	return &AppError{Code: ErrJnlPostingCodeMandatory, Table: table, Field: field, Params: []string{glAccNo}}
}

func JnlPostingCodeSame(table, field, code, glAccNo string) *AppError {
	return &AppError{Code: ErrJnlPostingCodeSame, Table: table, Field: field, Params: []string{code, glAccNo}}
}

func JnlPostingCodeBlank(table, field, glAccNo string) *AppError {
	return &AppError{Code: ErrJnlPostingCodeBlank, Table: table, Field: field, Params: []string{glAccNo}}
}

// JnlBankPmtCurrency: a bank payment type needs the bank account's currency on the line
func JnlBankPmtCurrency(table, field, bankAccNo string) *AppError {
	return &AppError{Code: ErrJnlBankPmtCurrency, Table: table, Field: field, Params: []string{bankAccNo}}
}

// JnlRequiredWhen: a field must be filled in when another has a value ("Document No. must be
// entered when Bank Payment Type is Manual Check")
func JnlRequiredWhen(table, whenField, field string, value any) *AppError {
	return &AppError{Code: ErrJnlRequiredWhen, Table: table, Field: whenField, Field2: field, Value: value}
}

// JnlCheckExists: a manual check number is already used for the bank account
func JnlCheckExists(checkNo string) *AppError {
	return &AppError{Code: ErrJnlCheckExists, Params: []string{checkNo}}
}

// JnlCheckPrintingNotYet: computer checks and electronic payments come with check printing
func JnlCheckPrintingNotYet(table, field string, value any) *AppError {
	return &AppError{Code: ErrJnlCheckPrintingNotYet, Table: table, Field: field, Value: value}
}

// CheckAlreadyExists: a check no. is printed, posted or financially voided for the bank account
func CheckAlreadyExists(checkNo string) *AppError {
	return &AppError{Code: ErrCheckAlreadyExists, Params: []string{checkNo}}
}

// CheckAmountTooLong: the amount in words does not fit the check's two lines
func CheckAmountTooLong(text string) *AppError {
	return &AppError{Code: ErrCheckAmountTooLong, Params: []string{text}}
}

// JnlNameTooLong: the batch name cannot be incremented within its length
func JnlNameTooLong(table, field string, length int) *AppError {
	return &AppError{Code: ErrJnlNameTooLong, Table: table, Field: field, Params: []string{fmt.Sprint(length)}}
}

// JnlCorrectionSame: Correction must be the same on all lines of a document
func JnlCorrectionSame(table, field string) *AppError {
	return &AppError{Code: ErrJnlCorrectionSame, Table: table, Field: field}
}

// JnlDocOutOfBalance: a document of the journal does not balance
func JnlDocOutOfBalance(table, docNo, amount string) *AppError {
	return &AppError{Code: ErrJnlDocOutOfBalance, Table: table, Field: "document_no", Field2: "posting_date", Params: []string{docNo, amount}}
}

// JnlAddCurrOutOfBalance: the lines of a document up to a line with another FA Add.-Currency
// Factor do not balance (CheckAddExchRateBalance)
func JnlAddCurrOutOfBalance(table, docNo string) *AppError {
	return &AppError{Code: ErrJnlAddCurrOutOfBalance, Table: table, Field: "document_no", Field2: "fa_add_currency_factor", Params: []string{docNo}}
}

// JnlDateOutOfBalance: the lines of a posting date do not balance
func JnlDateOutOfBalance(table, date, amount string) *AppError {
	return &AppError{Code: ErrJnlDateOutOfBalance, Table: table, Field: "posting_date", Field2: "amount", Params: []string{date, amount}}
}

// JnlDocOutOfBalanceCurrency / JnlDateOutOfBalanceCurrency: the same in the lines' currency
func JnlDocOutOfBalanceCurrency(table, docNo, amount, currency string) *AppError {
	return &AppError{Code: ErrJnlDocOutOfBalanceCurr, Table: table, Field: "document_no", Field2: "posting_date", Params: []string{docNo, amount, currency}}
}

func JnlDateOutOfBalanceCurrency(table, date, amount, currency string) *AppError {
	return &AppError{Code: ErrJnlDateOutOfBalanceCurr, Table: table, Field: "posting_date", Field2: "amount", Params: []string{date, amount, currency}}
}

// JnlTaxMultipleCustVend: a document with tax has more than one customer or vendor
func JnlTaxMultipleCustVend(table, docNo, date string) *AppError {
	return &AppError{Code: ErrJnlTaxMultipleCustVend, Table: table, Field: "document_no", Params: []string{docNo, date}}
}

// JnlMaxPostingNoSeries: too many posting number series in one journal
func JnlMaxPostingNoSeries(max int) *AppError {
	return &AppError{Code: ErrJnlMaxPostingNoSeries, Params: []string{fmt.Sprint(max)}}
}

// Customer and vendor entries and their application
const (
	ErrJnlSalesDocExists          ErrorCode = "ERR_JNL_SALES_DOC_EXISTS"
	ErrJnlPurchDocExists          ErrorCode = "ERR_JNL_PURCH_DOC_EXISTS"
	ErrJnlPurchDocExistsForVendor ErrorCode = "ERR_JNL_PURCH_DOC_EXISTS_FOR_VENDOR"
	ErrJnlAppliesToNotFound       ErrorCode = "ERR_JNL_APPLIES_TO_NOT_FOUND"
	ErrJnlApplnSameCurrency       ErrorCode = "ERR_JNL_APPLN_SAME_CURRENCY"
	ErrJnlCustPurchaseType        ErrorCode = "ERR_JNL_CUST_PURCHASE_TYPE"
	ErrJnlVendSalesType           ErrorCode = "ERR_JNL_VEND_SALES_TYPE"
	ErrJnlSpecifyOne              ErrorCode = "ERR_JNL_SPECIFY_ONE"
	ErrJnlCustOrVend              ErrorCode = "ERR_JNL_CUST_OR_VEND"
	ErrJnlUpdateInterrupted       ErrorCode = "ERR_JNL_UPDATE_INTERRUPTED"
)

// JnlSalesDocExists: a customer's invoice, credit memo, finance charge memo or reminder number
// is already posted
func JnlSalesDocExists(docType any, docNo string) *AppError {
	return &AppError{Code: ErrJnlSalesDocExists, Table: "Customer_ledger_entry", Field: "document_type", Params: []string{docNo}, Value: docType}
}

// JnlPurchDocExists: a vendor document number is already posted
func JnlPurchDocExists(docType any, docNo string) *AppError {
	return &AppError{Code: ErrJnlPurchDocExists, Table: "Vendor_ledger_entry", Field: "document_type", Params: []string{docNo}, Value: docType}
}

// JnlPurchDocExistsForVendor: the vendor's external document number is already posted
func JnlPurchDocExistsForVendor(docType any, externalDocNo string) *AppError {
	return &AppError{Code: ErrJnlPurchDocExistsForVendor, Table: "Vendor_ledger_entry", Field: "document_type", Params: []string{externalDocNo}, Value: docType}
}

// JnlAppliesToNotFound: no open entry has the line's Applies-to Doc. Type and No.
func JnlAppliesToNotFound(table string, docType any, docNo string) *AppError {
	return &AppError{Code: ErrJnlAppliesToNotFound, Table: table, Field: "document_type", Params: []string{docNo}, Value: docType}
}

// JnlApplnSameCurrency: all entries of one application must be in one currency
func JnlApplnSameCurrency() *AppError {
	return &AppError{Code: ErrJnlApplnSameCurrency}
}

// ErrJnlApplnSameCurrencyOrEMU: Appln. between Currencies EMU — the entries are in one currency or
// in EMU currencies
const ErrJnlApplnSameCurrencyOrEMU ErrorCode = "ERR_JNL_APPLN_SAME_CURRENCY_OR_EMU"

// JnlApplnSameCurrencyOrEMU: the entries of one application are in one currency or in EMU currencies
func JnlApplnSameCurrencyOrEMU() *AppError {
	return &AppError{Code: ErrJnlApplnSameCurrencyOrEMU}
}

// JnlCustPurchaseType / JnlVendSalesType: a customer line cannot post a purchase, a vendor
// line not a sale
func JnlCustPurchaseType() *AppError { return &AppError{Code: ErrJnlCustPurchaseType} }
func JnlVendSalesType() *AppError    { return &AppError{Code: ErrJnlVendSalesType} }

// JnlSpecifyOne: one of two fields must be filled in (Document No. or Applies-to ID)
func JnlSpecifyOne(table, field, field2 string) *AppError {
	return &AppError{Code: ErrJnlSpecifyOne, Table: table, Field: field, Field2: field2}
}

// JnlCustOrVend: Apply Entries needs a customer or vendor on the line
func JnlCustOrVend(table, field, field2 string) *AppError {
	return &AppError{Code: ErrJnlCustOrVend, Table: table, Field: field, Field2: field2}
}

// JnlUpdateInterrupted: the user did not confirm a change
func JnlUpdateInterrupted() *AppError { return &AppError{Code: ErrJnlUpdateInterrupted} }

// Sales tax
const (
	ErrSalesTaxCalculationOrder ErrorCode = "ERR_SALES_TAX_CALCULATION_ORDER"
	ErrSalesTaxAmountIncorrect  ErrorCode = "ERR_SALES_TAX_AMOUNT_INCORRECT"
	ErrTaxNotForSalesTax        ErrorCode = "ERR_TAX_NOT_FOR_SALES_TAX"
	ErrJnlUnrealizedTaxNotYet   ErrorCode = "ERR_JNL_UNREALIZED_TAX_NOT_YET"
	ErrJnlPmtDiscTaxAdjNotYet   ErrorCode = "ERR_JNL_PMT_DISC_TAX_ADJ_NOT_YET"
)

// SalesTaxCalculationOrder: the jurisdictions of a tax area with tax on tax need unique
// calculation orders
func SalesTaxCalculationOrder(taxArea string) *AppError {
	return &AppError{Code: ErrSalesTaxCalculationOrder, Table: "Tax_Area_Line", Field: "calculation_order", Params: []string{taxArea}}
}

// SalesTaxAmountIncorrect: the line's tax amount is not the tax of its area and group
func SalesTaxAmountIncorrect(taxArea, taxGroup string) *AppError {
	return &AppError{Code: ErrSalesTaxAmountIncorrect, Params: []string{taxArea, taxGroup}}
}

// TaxNotForSalesTax: a VAT field of a tax posting setup with Tax Calculation Type Sales Tax
func TaxNotForSalesTax(table, field string, value any) *AppError {
	return &AppError{Code: ErrTaxNotForSalesTax, Table: table, Field: "tax_calculation_type", Field2: field, Value: value}
}

// JnlUnrealizedTaxNotYet: unrealized tax cannot be posted yet
func JnlUnrealizedTaxNotYet() *AppError { return &AppError{Code: ErrJnlUnrealizedTaxNotYet} }

// JnlPmtDiscTaxAdjNotYet: a payment discount whose VAT is to be adjusted (Adjust for Payment
// Discount) cannot be posted yet; the document named
func JnlPmtDiscTaxAdjNotYet(documentNo string) *AppError {
	return &AppError{Code: ErrJnlPmtDiscTaxAdjNotYet, Params: []string{documentNo}}
}

// JnlTaxBaseFullVAT: a tax base cannot be entered with Full VAT
func JnlTaxBaseFullVAT(table, field string, value any) *AppError {
	return &AppError{Code: ErrJnlTaxBaseFullVAT, Table: table, Field: "tax_calculation_type", Field2: field, Value: value}
}

const ErrJnlTaxBaseFullVAT ErrorCode = "ERR_JNL_TAX_BASE_FULL_VAT"
