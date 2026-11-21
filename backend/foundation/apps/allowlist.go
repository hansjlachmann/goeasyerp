package apps

import (
	"fmt"
	"regexp"
	"strings"
)

// The repositories apps may be installed from (App Build Setup: Allowed Repositories): GitHub
// owners ("acme": all of its repositories) and repositories ("acme/loyalty"), separated by
// commas, semicolons or spaces, in any letter case. Empty: none — an installation installs
// apps from Git only from sources it named.

var allowedEntryRe = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})(?:/[A-Za-z0-9._-]{1,100})?$`)

// ParseAllowedRepositories splits the list into its entries (lower case) and refuses an entry
// that is not an owner or owner/repository
func ParseAllowedRepositories(list string) ([]string, error) {
	fields := strings.FieldsFunc(list, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' })
	entries := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(f), "https://github.com/"), "/")
		if !allowedEntryRe.MatchString(f) {
			return nil, fmt.Errorf("%q is not a GitHub owner or owner/repository", f)
		}
		entries = append(entries, strings.ToLower(f))
	}
	return entries, nil
}

// RepositoryAllowed reports whether the list allows the repository owner/name
func RepositoryAllowed(list, owner, name string) bool {
	entries, err := ParseAllowedRepositories(list)
	if err != nil {
		return false
	}
	owner, name = strings.ToLower(owner), strings.ToLower(name)
	for _, e := range entries {
		if e == owner || e == owner+"/"+name {
			return true
		}
	}
	return false
}
