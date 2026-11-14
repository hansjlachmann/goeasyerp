package main

// defaultCompany is the company a new, empty database starts with when COMPANY_NAME is not set.
const defaultCompany = "main"

// startupCompany decides the company the server enters at startup, from the company asked for
// (COMPANY_NAME, or the name typed at the prompt; blank: none) and the companies the database
// has (sorted by name). Entering a company only runs the migrations and the table sync — for every
// company — and gives the startup jobs (demo data) their company; each user's session works in the
// company the user chose. So:
//   - the company asked for is entered when it exists;
//   - otherwise an existing company is entered (the first by name): a database with companies is
//     never given an empty one because a name in a configuration file changed;
//   - an empty database gets the company asked for, or defaultCompany — create is true.
func startupCompany(requested string, existing []string) (name string, create bool) {
	for _, company := range existing {
		if company == requested {
			return requested, false
		}
	}
	if len(existing) > 0 {
		return existing[0], false
	}
	if requested != "" {
		return requested, true
	}
	return defaultCompany, true
}
