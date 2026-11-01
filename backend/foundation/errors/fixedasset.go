package errors

// Fixed asset posting errors (FA journals, codeunits 5600, 5606, 5631–5633). The messages about
// an asset name it and its depreciation book (classic FAName): their first two parameters.
const (
	ErrFAMustBeIdentical      ErrorCode = "ERR_FA_MUST_BE_IDENTICAL"
	ErrFAPostInGenJnl         ErrorCode = "ERR_FA_POST_IN_GEN_JNL"
	ErrFAPostInFAJnl          ErrorCode = "ERR_FA_POST_IN_FA_JNL"
	ErrFADeprGLIntegration    ErrorCode = "ERR_FA_DEPR_GL_INTEGRATION"
	ErrFANotTogetherWith      ErrorCode = "ERR_FA_NOT_TOGETHER_WITH"
	ErrFANotWithErrorEntry    ErrorCode = "ERR_FA_NOT_WITH_ERROR_ENTRY"
	ErrFANotDifferentThan     ErrorCode = "ERR_FA_NOT_DIFFERENT_THAN"
	ErrFANotYet               ErrorCode = "ERR_FA_NOT_YET"
	ErrFAMainAssetPosting     ErrorCode = "ERR_FA_MAIN_ASSET_POSTING"
	ErrFAFirstAcquisition     ErrorCode = "ERR_FA_FIRST_ACQUISITION"
	ErrFADisposed             ErrorCode = "ERR_FA_DISPOSED"
	ErrFAMustNotBePositive    ErrorCode = "ERR_FA_MUST_NOT_BE_POSITIVE"
	ErrFAMustNotBeNegative    ErrorCode = "ERR_FA_MUST_NOT_BE_NEGATIVE"
	ErrFABookValueNegative    ErrorCode = "ERR_FA_BOOK_VALUE_NEGATIVE"
	ErrFADeprBasisNegative    ErrorCode = "ERR_FA_DEPR_BASIS_NEGATIVE"
	ErrFAErrorEntryNotFound   ErrorCode = "ERR_FA_ERROR_ENTRY_NOT_FOUND"
	ErrFAErrorEntryNoMatch    ErrorCode = "ERR_FA_ERROR_ENTRY_NO_MATCH"
)

// FAMustBeIdentical: a date must be the same as another (the book's Use Same FA+G/L Posting
// Dates)
func FAMustBeIdentical(table, field, field2 string) *AppError {
	return &AppError{Code: ErrFAMustBeIdentical, Table: table, Field: field, Field2: field2}
}

// FAPostInGenJnl: the posting type is G/L-integrated in the book: it is posted in the general
// journal (value: the posting type)
func FAPostInGenJnl(table, field string, value any) *AppError {
	return &AppError{Code: ErrFAPostInGenJnl, Table: table, Field: field, Value: value}
}

// FAPostInFAJnl: the posting type is not G/L-integrated in the book: it is posted in the FA
// journal (value: the posting type)
func FAPostInFAJnl(table, field string, value any) *AppError {
	return &AppError{Code: ErrFAPostInFAJnl, Table: table, Field: field, Value: value}
}

// FADeprGLIntegration: a field must not be used when the book's G/L Integration - Depreciation
// is (or is not) on
func FADeprGLIntegration(table, field, deprBookCode string, integration bool) *AppError {
	return &AppError{Code: ErrFADeprGLIntegration, Table: table, Field: field, Params: []string{deprBookCode}, Value: integration}
}

// FANotTogetherWith: a field must not be filled in with the line's posting type (field2 = the
// posting type field, value: its value)
func FANotTogetherWith(table, field, field2 string, value any) *AppError {
	return &AppError{Code: ErrFANotTogetherWith, Table: table, Field: field, Field2: field2, Value: value}
}

// FANotWithErrorEntry: a field must not be filled in on a line that cancels an entry (FA Error
// Entry No.)
func FANotWithErrorEntry(table, field, field2 string) *AppError {
	return &AppError{Code: ErrFANotWithErrorEntry, Table: table, Field: field, Field2: field2}
}

// FANotDifferentThan: a field must differ from another (Duplicate in Depreciation Book)
func FANotDifferentThan(table, field, field2 string) *AppError {
	return &AppError{Code: ErrFANotDifferentThan, Table: table, Field: field, Field2: field2}
}

// FANotYet: a field or posting type that comes with a later fixed asset step (value: the
// field's value — a posting type)
func FANotYet(table, field string, value any) *AppError {
	return &AppError{Code: ErrFANotYet, Table: table, Field: field, Value: value}
}

// FAMainAssetPosting: a main asset cannot be posted to without the FA Setup's Allow Posting to
// Main Assets
func FAMainAssetPosting(faNo, deprBookCode string) *AppError {
	return &AppError{Code: ErrFAMainAssetPosting, Params: []string{faNo, deprBookCode}}
}

// FAFirstAcquisition: an asset's first entry in a book must be an acquisition cost
func FAFirstAcquisition(faNo, deprBookCode string) *AppError {
	return &AppError{Code: ErrFAFirstAcquisition, Params: []string{faNo, deprBookCode}}
}

// FADisposed: the asset is disposed in the book
func FADisposed(faNo, deprBookCode string) *AppError {
	return &AppError{Code: ErrFADisposed, Params: []string{faNo, deprBookCode}}
}

// FAMustNotBeSigned: the posting type's total (accumulated: depreciation) must not become
// positive / negative on a date (table/field: the FA ledger entry's posting type, value: the
// type)
func FAMustNotBeSigned(positive bool, faNo, deprBookCode, date string, table, field string, value any) *AppError {
	code := ErrFAMustNotBeNegative
	if positive {
		code = ErrFAMustNotBePositive
	}
	return &AppError{Code: code, Table: table, Field: field, Params: []string{faNo, deprBookCode, date}, Value: value}
}

// FABookValueNegative: the book value must not be negative or less than the salvage value
func FABookValueNegative(faNo, deprBookCode, date string) *AppError {
	return &AppError{Code: ErrFABookValueNegative, Params: []string{faNo, deprBookCode, date}}
}

// FADeprBasisNegative: the depreciable basis must not be negative
func FADeprBasisNegative(faNo, deprBookCode, date string) *AppError {
	return &AppError{Code: ErrFADeprBasisNegative, Params: []string{faNo, deprBookCode, date}}
}

// FAErrorEntryNotFound: the entry a line cancels does not exist
func FAErrorEntryNotFound(faNo, deprBookCode, entryNo string) *AppError {
	return &AppError{Code: ErrFAErrorEntryNotFound, Params: []string{faNo, deprBookCode, entryNo}}
}

// FAErrorEntryNoMatch: the entry a line cancels is not its opposite (asset, book, posting type,
// amount, date)
func FAErrorEntryNoMatch(faNo, deprBookCode, entryNo string) *AppError {
	return &AppError{Code: ErrFAErrorEntryNoMatch, Params: []string{faNo, deprBookCode, entryNo}}
}

// ErrFANoSpaceForBalAccounts: Insert FA Bal. Account finds no free line numbers below the line
const ErrFANoSpaceForBalAccounts ErrorCode = "ERR_FA_NO_SPACE_FOR_BAL_ACCOUNTS"

// FANoSpaceForBalAccounts: there is not enough space to insert the balancing accounts
func FANoSpaceForBalAccounts() *AppError {
	return &AppError{Code: ErrFANoSpaceForBalAccounts}
}
