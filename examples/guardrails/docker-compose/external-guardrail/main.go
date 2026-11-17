package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

// main serves a mock external guardrail that logs every guardrails.Input the gateway
// POSTs and answers {"action", "message"}. With BLOCK set it refuses every call, which
// the gateway honours because only an external block overrides the Rego decision.
func main() {
	block := os.Getenv("BLOCK") != ""

	mux := http.NewServeMux()
	mux.HandleFunc("POST /check", func(w http.ResponseWriter, r *http.Request) {
		var input map[string]any
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		seen, _ := json.Marshal(input)
		log.Printf("guardrail check: %s", seen)

		decision := map[string]string{"action": "allow"}
		if block {
			decision = map[string]string{
				"action":  "block",
				"message": "refused by the external guardrail service",
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(decision)
	})

	log.Printf("external guardrail service listening on :8088 (BLOCK=%v)", block)
	log.Fatal(http.ListenAndServe(":8088", mux))
}
