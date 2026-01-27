package routes

import "github.com/go-chi/chi/v5"

func PublicRouter() chi.Router {
	r := chi.NewRouter()

	r.Get("/status", status)

	return r
}
