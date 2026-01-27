package respond

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Error().Err(err).Interface("data", data).Msg("Failed to encode JSON")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}

func Text(w http.ResponseWriter, status int, data string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(status)
	w.Write([]byte(data))
}

func Error(w http.ResponseWriter, status int, message string, data any) {
	if data != nil {
		JSON(w, status, map[string]any{"error": message, "data": data})
	} else {
		JSON(w, status, map[string]string{"error": message})
	}
}
