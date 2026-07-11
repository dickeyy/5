package routes

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/httpapi/middleware"
	"github.com/quackdiscord/bot/internal/quack"
)

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
	if err := c.ShouldBindJSON(&input); err != nil {
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
func updateTemplate(c *gin.Context, services *quack.Services) {
	var input quack.TemplateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid template payload"})
		return
	}

	template, err := services.Templates.Update(c.Request.Context(), middleware.GetGuildContext(c), c.Param("templateID"), input)
	if err != nil {
		writeTemplateError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"template": template})
}

// archiveTemplate encapsulates the archive template rule so callers share one consistent package implementation.
func archiveTemplate(c *gin.Context, services *quack.Services) {
	template, err := services.Templates.Archive(c.Request.Context(), middleware.GetGuildContext(c), c.Param("templateID"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"template": template})
}

// writeTemplateError maps template error into the preserved HTTP error response contract.
func writeTemplateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, quack.ErrTemplateValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, quack.ErrTemplateNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "template operation failed"})
	}
}
