package routes

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/httpapi/middleware"
	"github.com/quackdiscord/bot/internal/quack"
)

// templateChangeHandler receives successful policy changes that may invalidate
// an optional automation reference.
type templateChangeHandler interface {
	HandleTemplateChange(context.Context, string, string)
}

// listTemplates returns templates subject to authorization, ordering, and filtering constraints.
func listTemplates(c *gin.Context, services *quack.Services) {
	guildContext := middleware.GetGuildContext(c)
	templates, err := services.Templates.List(c.Request.Context(), guildContext)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list templates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// createTemplate creates template while preserving validation, authorization, and persistence invariants.
func createTemplate(c *gin.Context, services *quack.Services) {
	var input quack.TemplateInput
	if err := bindTemplateInput(c, &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template payload"})
		return
	}

	template, err := services.Templates.Create(c.Request.Context(), middleware.GetGuildContext(c), input)
	if err != nil {
		writeTemplateError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"template": template})
}

// getTemplate retrieves template without exposing the underlying adapter implementation.
func getTemplate(c *gin.Context, services *quack.Services) {
	template, err := services.Templates.Get(c.Request.Context(), middleware.GetGuildContext(c), c.Param("templateID"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"template": template})
}

// updateTemplate updates template while retaining validation, compatibility, and audit requirements.
func updateTemplate(c *gin.Context, services *quack.Services, changes templateChangeHandler) {
	var input quack.TemplateInput
	if err := bindTemplateInput(c, &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template payload"})
		return
	}

	template, err := services.Templates.Update(c.Request.Context(), middleware.GetGuildContext(c), c.Param("templateID"), input)
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	if changes != nil {
		guildContext := middleware.GetGuildContext(c)
		if guildContext != nil && guildContext.Guild != nil {
			changes.HandleTemplateChange(c.Request.Context(), guildContext.Guild.ID, template.ID)
		}
	}

	c.JSON(http.StatusOK, gin.H{"template": template})
}

// bindTemplateInput rejects retired or unknown product fields instead of silently ignoring them.
func bindTemplateInput(c *gin.Context, input *quack.TemplateInput) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

// archiveTemplate encapsulates the archive template rule so callers share one consistent package implementation.
func archiveTemplate(c *gin.Context, services *quack.Services, changes templateChangeHandler) {
	template, err := services.Templates.Archive(c.Request.Context(), middleware.GetGuildContext(c), c.Param("templateID"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	if changes != nil {
		guildContext := middleware.GetGuildContext(c)
		if guildContext != nil && guildContext.Guild != nil {
			changes.HandleTemplateChange(c.Request.Context(), guildContext.Guild.ID, template.ID)
		}
	}

	c.JSON(http.StatusOK, gin.H{"template": template})
}

// restoreTemplate reverses archive without creating a new template identity.
func restoreTemplate(c *gin.Context, services *quack.Services) {
	template, err := services.Templates.Restore(c.Request.Context(), middleware.GetGuildContext(c), c.Param("templateID"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"template": template})
}

// exportTemplate returns guild-neutral moderation policy only.
func exportTemplate(c *gin.Context, services *quack.Services) {
	policy, err := services.Templates.Export(c.Request.Context(), middleware.GetGuildContext(c), c.Param("templateID"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"policy": policy})
}

// importTemplate requires explicit confirmation before activating a new guild-owned identity.
func importTemplate(c *gin.Context, services *quack.Services) {
	var input quack.TemplateImportInput
	if err := decodeStrictJSON(c, &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid import payload"})
		return
	}
	template, err := services.Templates.Import(c.Request.Context(), middleware.GetGuildContext(c), input)
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"template": template})
}

// writeTemplateError maps template error into the preserved HTTP error response contract.
func writeTemplateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, quack.ErrTemplateValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, quack.ErrTemplateNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, quack.ErrTemplateCompatibilityReviewRequired):
		var compatibilityError *quack.TemplateCompatibilityReviewError
		if errors.As(err, &compatibilityError) {
			c.JSON(http.StatusConflict, gin.H{
				"error":                quack.ErrTemplateCompatibilityReviewRequired.Error(),
				"template_id":          compatibilityError.TemplateID,
				"compatibility_reason": compatibilityError.Reason,
			})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": quack.ErrTemplateCompatibilityReviewRequired.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "template operation failed"})
	}
}
