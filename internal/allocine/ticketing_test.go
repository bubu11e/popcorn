package allocine

import (
	"encoding/json"
	"testing"
)

func TestTicketingURL(t *testing.T) {
	cases := []struct {
		name string
		json string
		want string
	}{
		{"no data", `{"startsAt": "2026-05-29T14:10:00"}`, ""},
		{"first non-empty wins",
			`{"data": {"ticketing": [{"urls": ["https://book/a"]}, {"urls": ["https://book/b"]}]}}`,
			"https://book/a"},
		{"skips empty url",
			`{"data": {"ticketing": [{"urls": ["", "https://book/b"]}]}}`,
			"https://book/b"},
		{"all empty", `{"data": {"ticketing": [{"urls": [""]}]}}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var s apiSeance
			if err := json.Unmarshal([]byte(tc.json), &s); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got := s.ticketingURL(); got != tc.want {
				t.Errorf("ticketingURL() = %q, want %q", got, tc.want)
			}
		})
	}
}
