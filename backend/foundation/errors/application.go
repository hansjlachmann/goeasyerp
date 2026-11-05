package errors

import "strconv"

// Applying and unapplying posted customer and vendor entries (codeunits 226/227)
const (
	ErrApplyNothingToApply     ErrorCode = "ERR_APPLY_NOTHING_TO_APPLY"
	ErrUnapplyNoApplication    ErrorCode = "ERR_UNAPPLY_NO_APPLICATION"
	ErrUnapplyLaterApplication ErrorCode = "ERR_UNAPPLY_LATER_APPLICATION"
)

// ApplyNothingToApply: Post Application found no other open entry marked with the entry's
// Applies-to ID
func ApplyNothingToApply(entryNo int) *AppError {
	return &AppError{Code: ErrApplyNothingToApply, Params: []string{strconv.Itoa(entryNo)}}
}

// UnapplyNoApplication: the entry has no application to unapply
func UnapplyNoApplication(entryNo int) *AppError {
	return &AppError{Code: ErrUnapplyNoApplication, Params: []string{strconv.Itoa(entryNo)}}
}

// UnapplyLaterApplication: an entry of the application was applied again later — that
// application is unapplied first
func UnapplyLaterApplication(entryNo, laterApplicationNo int) *AppError {
	return &AppError{Code: ErrUnapplyLaterApplication, Params: []string{strconv.Itoa(entryNo), strconv.Itoa(laterApplicationNo)}}
}
