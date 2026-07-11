package store

import (
	"time"

	"github.com/quackdiscord/bot/internal/quack/idutil"
	"github.com/quackdiscord/bot/internal/quack/model"
)

// prepareULIDModel encapsulates the prepare ulidmodel rule so callers share one consistent package implementation.
func prepareULIDModel(model *model.ULIDModel, now time.Time) error {
	if model.ID == "" {
		id, err := idutil.NewULID()
		if err != nil {
			return err
		}
		model.ID = id
	}

	if model.CreatedAt.IsZero() {
		model.CreatedAt = now
	}
	model.UpdatedAt = now

	return nil
}
