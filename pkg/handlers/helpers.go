package handlers

import (
	"encoding/json"
	"net/http"
)

func writeJSONResponse(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	// By default, decide whether or not a cache is usable based on the matching of the JWT
	// For example, this will keep caches from being used in the same browser if two users were to log in back to back
	w.Header().Set("Vary", "Authorization")

	w.WriteHeader(code)

	if payload != nil {
		response, _ := json.Marshal(payload)
		_, _ = w.Write(response)
	}
}
