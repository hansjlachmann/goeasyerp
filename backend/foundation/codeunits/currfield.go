package codeunits

import "sync"

// currentField holds the field the user is entering (per goroutine): the classic CurrFieldNo —
// a trigger runs because the user changed that field, not because code validated it
var currentField sync.Map

// SetCurrFieldNo marks the field the user enters while its OnValidate triggers run (the API's
// validate, insert and modify); 0 ends it
func SetCurrFieldNo(fieldNo int) {
	if fieldNo == 0 {
		currentField.Delete(getGoroutineID())
		return
	}
	currentField.Store(getGoroutineID(), fieldNo)
}

// CurrFieldNo is the number of the field the user is entering (classic CurrFieldNo): 0 when the
// trigger runs from code (a posting, a codeunit, demo data)
func CurrFieldNo() int {
	v, ok := currentField.Load(getGoroutineID())
	if !ok {
		return 0
	}
	return v.(int)
}
