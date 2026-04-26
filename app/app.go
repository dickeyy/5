package app

import "github.com/quackdiscord/bot/storage"

// Services is the application service boundary shared by API routes and
// future Discord command adapters. Business use cases should be added here
// instead of being implemented directly in handlers.
type Services struct {
	Store *storage.Store
}

func New(store *storage.Store) *Services {
	return &Services{Store: store}
}
