package tables

import (
	"testing"

	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

func TestGetTrackingInternetAddr(t *testing.T) {
	for _, c := range []struct{ address, want string }{
		{"track.example.com/?id=%1", "https://track.example.com/?id=PKG1"},
		{"http://track.example.com/%1", "http://track.example.com/PKG1"},
		{"HTTPS://track.example.com/%1/x", "HTTPS://track.example.com/PKG1/x"},
	} {
		shippingAgent := &gtables.ShippingAgent{Internet_address: types.Text(c.address)}
		got := GetTrackingInternetAddr(shippingAgent, "PKG1")
		if got != c.want {
			t.Errorf("%q: %q, want %q", c.address, got, c.want)
		}
	}
}
