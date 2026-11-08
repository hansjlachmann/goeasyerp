package middleware

import "testing"

func TestAcceptedLanguage(t *testing.T) {
	supported := []string{"en-US", "nb-NO", "da-DK"}
	cases := map[string]string{
		"":                        "",
		"nb-NO,nb;q=0.9,en;q=0.8": "nb-NO",
		"de-DE,da;q=0.9":          "da-DK",
		"no":                      "nb-NO",
		"en-GB,en;q=0.9":          "en-US",
		"fr-FR":                   "",
		"en;q=0.5, da-DK;q=0.9":   "da-DK",
		"*":                       "",
		"da-dk":                   "da-DK",
	}
	for header, want := range cases {
		got := acceptedLanguage(header, append([]string(nil), supported...))
		if got != want {
			t.Errorf("acceptedLanguage(%q) = %q, want %q", header, got, want)
		}
	}
}
