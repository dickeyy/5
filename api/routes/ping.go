package routes

import (
	"net/http"

	"github.com/quackdiscord/bot/api/respond"
)

func ping(w http.ResponseWriter, r *http.Request) {
	respond.Text(w, http.StatusOK, "pong")
}
