package handlers

import (
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
)

// Input dialog answers may hold numbers (number inputs): they reach the codeunit as text
func TestInputAnswerWithNumbers(t *testing.T) {
	d := fcodeunits.OpenDialogWithID("", "input-number-test")
	answer := make(chan map[string]string, 1)
	go func() {
		answer <- d.RequestInput("Options", []fcodeunits.InputField{{Name: "days", Type: "number"}})
	}()
	ev := <-d.Events()

	app := fiber.New()
	app.Post("/api/jobs/:id/confirm", (&JobsHandler{}).RespondToConfirm)
	out := postJSON(t, app, "/api/jobs/input-number-test/confirm",
		fmt.Sprintf(`{"confirm_id":%q,"values":{"days":30,"filter":"J001","flag":true,"empty":null}}`, ev.ConfirmID))
	if out["success"] != true {
		t.Fatalf("answer refused: %v", out)
	}
	values := <-answer
	if values["days"] != "30" || values["filter"] != "J001" || values["flag"] != "true" || values["empty"] != "" {
		t.Errorf("values %v", values)
	}
}
