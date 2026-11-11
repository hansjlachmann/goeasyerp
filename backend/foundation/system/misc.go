package system

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
)

// Random returns a random whole number from 1 to n (1 when n < 1)
func Random(n int) int {
	if n < 1 {
		return 1
	}
	r, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 1
	}
	return int(r.Int64()) + 1
}

// CreateGuid returns a new random GUID (version 4) in lower case:
// "6f1c…-….-4…-….-…………"
func CreateGuid() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40 // version 4
	b[8] = b[8]&0x3f | 0x80 // variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

var guidPattern = regexp.MustCompile(`^\{?[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\}?$`)

// IsNullGuid reports whether a GUID is blank or all zeros (also when it is not a GUID)
func IsNullGuid(guid string) bool {
	guid = strings.TrimSpace(guid)
	if !guidPattern.MatchString(guid) {
		return true
	}
	return strings.Trim(guid, "{}0-") == ""
}

// UserId returns the user the code runs for (the session's user in a codeunit)
func UserId() string {
	return codeunits.CurrentUserID()
}

// Confirm asks the user a yes/no question in a codeunit (the text filled in with StrSubstNo;
// pass a translated text) and returns the answer; without a dialog (a scheduled run) it is
// false
func Confirm(text string, values ...any) bool {
	return codeunits.Confirm(StrSubstNo(text, values...))
}

// StrMenu shows a menu of options (a comma-separated string, like SelectStr's: "Ship,Invoice,
// Ship and Invoice") for the user to pick one in a codeunit and returns its number (1-based);
// 0 when the user cancels, or without a dialog (a scheduled run). defaultNumber is the option
// selected first (0 or out of range: the first one); instruction (optional, translated) is the
// text above the options.
func StrMenu(options string, defaultNumber int, instruction ...string) int {
	items := strings.Split(options, ",")
	if strings.TrimSpace(options) == "" {
		return 0
	}
	if defaultNumber < 1 || defaultNumber > len(items) {
		defaultNumber = 1
	}
	title := ""
	if len(instruction) > 0 {
		title = instruction[0]
	}
	values := codeunits.RequestInput(title, []codeunits.InputField{{
		Name:     "option",
		Type:     "option",
		Required: true,
		Default:  strconv.Itoa(defaultNumber),
		Options:  items,
	}})
	n, err := strconv.Atoi(values["option"])
	if err != nil || n < 1 || n > len(items) {
		return 0
	}
	return n
}

// Hyperlink opens a URL (http, https or mailto) in the user's browser, in a new tab, from a
// codeunit; without a dialog (a scheduled run) nothing happens. Another kind of URL is an error.
func Hyperlink(link string) error {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return fmt.Errorf("Hyperlink: %q is not an http, https or mailto URL", link)
	}
	web := (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
	mail := u.Scheme == "mailto" && u.Opaque != ""
	if !web && !mail {
		return fmt.Errorf("Hyperlink: %q is not an http, https or mailto URL", link)
	}
	codeunits.Hyperlink(u.String())
	return nil
}

// CanConfirm reports whether Confirm can ask the user: in a codeunit's dialog, or in an API
// request — a table trigger run by the user's change, its question answered in rounds (the
// request is sent again with the answer). A question in a request is no place for RunModal,
// StrMenu or RequestInput: check GuiAllowed for those.
func CanConfirm() bool {
	return GuiAllowed() || codeunits.CanAskQuestions()
}

// GuiAllowed reports whether the code runs with a user to ask (a codeunit's dialog), not in a
// scheduled run: ask with Confirm, StrMenu or RunModal only then
func GuiAllowed() bool {
	return codeunits.GetCurrentDialog() != nil
}
