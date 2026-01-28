package structs

type QueueEvent struct {
	Type    string
	Data    any
	Handler func(data any)
}
