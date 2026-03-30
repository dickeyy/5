package structs

import "github.com/quackdiscord/bot/storage"

type QueueEvent struct {
	Type    string
	Data    any
	Handler func(s *storage.Store, data any)
}
