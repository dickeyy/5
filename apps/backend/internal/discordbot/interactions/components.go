package interactions

import (
	"errors"
	"strings"

	"github.com/quackdiscord/bot/internal/discordbot/ui"
)

var ErrComponentHandlerNotFound = errors.New("component handler not found")

// ComponentRegistry maps custom-ID namespaces to component and modal handlers without global registration.
type ComponentRegistry struct {
	components map[string]ui.Handler
	modals     map[string]ui.Handler
}

// NewComponentRegistry constructs component registry with required dependencies explicit so callers control lifecycle and substitution.
func NewComponentRegistry() *ComponentRegistry {
	return &ComponentRegistry{
		components: map[string]ui.Handler{},
		modals:     map[string]ui.Handler{},
	}
}

// RegisterComponent explicitly wires register component so runtime behavior does not depend on init-time registration.
func (r *ComponentRegistry) RegisterComponent(namespace, action string, handler ui.Handler) error {
	return r.register(r.components, namespace, action, handler)
}

// RegisterModal explicitly wires register modal so runtime behavior does not depend on init-time registration.
func (r *ComponentRegistry) RegisterModal(namespace, action string, handler ui.Handler) error {
	return r.register(r.modals, namespace, action, handler)
}

// LookupComponent encapsulates the lookup component rule so callers share one consistent package implementation.
func (r *ComponentRegistry) LookupComponent(customID string) (ui.Handler, bool, error) {
	return r.lookup(r.components, customID)
}

// LookupModal encapsulates the lookup modal rule so callers share one consistent package implementation.
func (r *ComponentRegistry) LookupModal(customID string) (ui.Handler, bool, error) {
	return r.lookup(r.modals, customID)
}

// register encapsulates the register rule so callers share one consistent package implementation.
func (r *ComponentRegistry) register(target map[string]ui.Handler, namespace, action string, handler ui.Handler) error {
	if r == nil {
		return errors.New("component registry is not configured")
	}
	if handler == nil {
		return errors.New("component handler is required")
	}
	key := Key(namespace, action)
	if key == ":" || strings.HasPrefix(key, ":") || strings.HasSuffix(key, ":") {
		return errors.New("component namespace and action are required")
	}
	if _, exists := target[key]; exists {
		return errors.New("component handler is already registered")
	}
	target[key] = handler
	return nil
}

// lookup encapsulates the lookup rule so callers share one consistent package implementation.
func (r *ComponentRegistry) lookup(source map[string]ui.Handler, customID string) (ui.Handler, bool, error) {
	if r == nil {
		return nil, false, ErrComponentHandlerNotFound
	}
	parsed, err := ui.DecodeCustomID(customID)
	if err != nil {
		return nil, false, err
	}
	handler, ok := source[Key(parsed.Namespace, parsed.Action)]
	return handler, ok, nil
}
