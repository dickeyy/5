package routes

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/httpapi/apierror"
	"github.com/quackdiscord/bot/internal/httpapi/middleware"
	httpplatform "github.com/quackdiscord/bot/internal/httpapi/platform"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
)

// RegisterAppealAndMemberRoutes mounts target-owned case and appeal routes on an authenticated member group.
func RegisterAppealAndMemberRoutes(group *gin.RouterGroup, services *quack.Services, appeals *quack.AppealService, primitives httpplatform.Primitives) error {
	if group == nil || services == nil || appeals == nil || primitives.RateLimits == nil || primitives.Idempotency == nil {
		return errors.New("appeal member route dependencies are not configured")
	}
	member := group.Group("")
	member.Use(primitives.RateLimits.Limit("member-reads-and-appeals", memberReadLimit(services), memberSubject))
	member.GET("/guilds/:guildID/cases", func(c *gin.Context) { listMemberOwnedCases(c, services) })
	member.GET("/cases/:caseID", func(c *gin.Context) { getMemberOwnedCase(c, services) })
	member.POST("/cases/:caseID/appeal", memberWriteIdempotency(primitives, services, "appeal-submit"), func(c *gin.Context) { submitAppeal(c, appeals) })
	member.GET("/appeals/:appealID", func(c *gin.Context) { getMemberAppeal(c, appeals) })
	member.POST("/appeals/:appealID/information", memberWriteIdempotency(primitives, services, "appeal-information"), func(c *gin.Context) { submitAppealInformation(c, appeals) })
	return nil
}

// RegisterAppealStaffRoutes mounts appeal settings, queue, decisions, and explicit reversal routes on an authenticated guild group.
func RegisterAppealStaffRoutes(group *gin.RouterGroup, services *quack.Services, appeals *quack.AppealService, primitives httpplatform.Primitives) error {
	if group == nil || services == nil || appeals == nil || primitives.RateLimits == nil || primitives.Idempotency == nil {
		return errors.New("appeal staff route dependencies are not configured")
	}
	staff := group.Group("/:discordGuildID")
	staff.Use(middleware.RequireGuildContext(services, ""))
	staff.Use(primitives.RateLimits.Limit("appeal-staff", memberReadLimit(services), staffAppealSubject))
	staff.GET("/appeal-settings", func(c *gin.Context) { getAppealSettings(c, appeals) })
	staff.PUT("/appeal-settings", staffWriteIdempotency(primitives, services, "appeal-settings"), func(c *gin.Context) { updateAppealSettings(c, appeals) })
	staff.GET("/appeals", func(c *gin.Context) { listStaffAppeals(c, appeals) })
	staff.GET("/appeals/:appealID", func(c *gin.Context) { getStaffAppeal(c, appeals) })
	staff.POST("/appeals/:appealID/request-information", staffWriteIdempotency(primitives, services, "appeal-request-information"), func(c *gin.Context) { requestAppealInformation(c, appeals) })
	staff.POST("/appeals/:appealID/reopen", staffWriteIdempotency(primitives, services, "appeal-reopen"), func(c *gin.Context) { reopenAppeal(c, appeals) })
	staff.POST("/appeals/:appealID/accept", staffWriteIdempotency(primitives, services, "appeal-accept"), func(c *gin.Context) { acceptAppeal(c, appeals) })
	staff.POST("/appeals/:appealID/reject", staffWriteIdempotency(primitives, services, "appeal-reject"), func(c *gin.Context) { rejectAppeal(c, appeals) })
	staff.POST("/appeals/:appealID/close", staffWriteIdempotency(primitives, services, "appeal-close"), func(c *gin.Context) { closeAppeal(c, appeals) })
	staff.POST("/appeals/:appealID/reversals", staffWriteIdempotency(primitives, services, "appeal-reversal"), func(c *gin.Context) { reverseAcceptedAppeal(c, services, appeals) })
	return nil
}

// submitAppeal creates the member's single appeal for an eligible case.
// @Summary Submit a case appeal
// @Tags Appeals
// @Accept json
// @Produce json
// @Param caseID path string true "Case ID"
// @Param Idempotency-Key header string true "Retry-safe request key"
// @Param appeal body quack.AppealSubmissionInput true "Appeal answers"
// @Security CookieAuth
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Router /members/me/cases/{caseID}/appeal [post]
func submitAppeal(c *gin.Context, appeals *quack.AppealService) {
	session := middleware.GetAuthSession(c)
	if session == nil {
		apierror.Write(c, http.StatusUnauthorized, apierror.CodeAuthentication, "authentication required")
		return
	}
	var input quack.AppealSubmissionInput
	if err := decodeStrictJSON(c, &input); err != nil {
		apierror.Write(c, http.StatusBadRequest, apierror.CodeValidation, "invalid appeal payload")
		return
	}
	result, err := appeals.Submit(c.Request.Context(), c.Param("caseID"), session.DiscordUserID, input)
	if err != nil {
		writeAppealError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"appeal": result})
}

// getMemberAppeal returns an appeal only to the member who owns it.
// @Summary Get the current member's appeal
// @Tags Appeals
// @Produce json
// @Param appealID path string true "Appeal ID"
// @Security CookieAuth
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Router /members/me/appeals/{appealID} [get]
func getMemberAppeal(c *gin.Context, appeals *quack.AppealService) {
	session := middleware.GetAuthSession(c)
	if session == nil {
		apierror.Write(c, http.StatusUnauthorized, apierror.CodeAuthentication, "authentication required")
		return
	}
	result, err := appeals.GetMember(c.Request.Context(), c.Param("appealID"), session.DiscordUserID)
	if err != nil {
		writeAppealError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"appeal": result})
}

// submitAppealInformation appends a member response requested by staff.
// @Summary Add requested appeal information
// @Tags Appeals
// @Accept json
// @Produce json
// @Param appealID path string true "Appeal ID"
// @Param Idempotency-Key header string true "Retry-safe request key"
// @Param information body quack.AppealInformationInput true "Additional information"
// @Security CookieAuth
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} apierror.Response
// @Failure 401 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Router /members/me/appeals/{appealID}/information [post]
func submitAppealInformation(c *gin.Context, appeals *quack.AppealService) {
	session := middleware.GetAuthSession(c)
	if session == nil {
		apierror.Write(c, http.StatusUnauthorized, apierror.CodeAuthentication, "authentication required")
		return
	}
	var input quack.AppealInformationInput
	if err := decodeStrictJSON(c, &input); err != nil {
		apierror.Write(c, http.StatusBadRequest, apierror.CodeValidation, "invalid appeal information payload")
		return
	}
	result, err := appeals.SubmitInformation(c.Request.Context(), c.Param("appealID"), session.DiscordUserID, input)
	if err != nil {
		writeAppealError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"appeal": result})
}

// getAppealSettings returns the effective appeal form for a guild.
// @Summary Get guild appeal settings
// @Tags Appeals
// @Produce json
// @Param discordGuildID path string true "Discord guild ID"
// @Security CookieAuth
// @Success 200 {object} quack.AppealSettingsResponse
// @Failure 403 {object} apierror.Response
// @Router /guilds/{discordGuildID}/appeal-settings [get]
func getAppealSettings(c *gin.Context, appeals *quack.AppealService) {
	guildContext := middleware.GetGuildContext(c)
	if guildContext == nil || guildContext.Guild == nil || !guildContext.Can(model.PermissionActionGuildSettingsRead) {
		writeAppealError(c, quack.ErrAppealPermissionDenied)
		return
	}
	result, err := appeals.GetSettings(c.Request.Context(), guildContext.Guild.ID)
	if err != nil {
		writeAppealError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type appealSettingsRequest struct {
	Questions []model.AppealQuestion `json:"questions"`
}

// updateAppealSettings replaces the form snapshotted by future appeals.
// @Summary Update guild appeal settings
// @Tags Appeals
// @Accept json
// @Produce json
// @Param discordGuildID path string true "Discord guild ID"
// @Param Idempotency-Key header string true "Retry-safe request key"
// @Param settings body appealSettingsRequest true "Appeal questions"
// @Security CookieAuth
// @Success 200 {object} quack.AppealSettingsResponse
// @Failure 400 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Router /guilds/{discordGuildID}/appeal-settings [put]
func updateAppealSettings(c *gin.Context, appeals *quack.AppealService) {
	var input appealSettingsRequest
	if err := decodeStrictJSON(c, &input); err != nil {
		apierror.Write(c, http.StatusBadRequest, apierror.CodeValidation, "invalid appeal settings payload")
		return
	}
	result, err := appeals.UpdateSettings(c.Request.Context(), middleware.GetGuildContext(c), input.Questions)
	if err != nil {
		writeAppealError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// listStaffAppeals returns the guild-scoped appeal review queue.
// @Summary List guild appeals
// @Tags Appeals
// @Produce json
// @Param discordGuildID path string true "Discord guild ID"
// @Param status query string false "Appeal status"
// @Param limit query int false "Page size"
// @Param offset query int false "Page offset"
// @Security CookieAuth
// @Success 200 {object} quack.AppealListResponse
// @Failure 400 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Router /guilds/{discordGuildID}/appeals [get]
func listStaffAppeals(c *gin.Context, appeals *quack.AppealService) {
	limit, offset, err := parsePageInts(c.Query("limit"), c.Query("offset"))
	if err != nil {
		apierror.Write(c, http.StatusBadRequest, apierror.CodeValidation, "invalid pagination")
		return
	}
	result, err := appeals.ListStaff(c.Request.Context(), middleware.GetGuildContext(c), model.AppealStatus(strings.TrimSpace(c.Query("status"))), limit, offset)
	if err != nil {
		writeAppealError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// getStaffAppeal returns one appeal to authorized guild staff.
// @Summary Get a guild appeal
// @Tags Appeals
// @Produce json
// @Param discordGuildID path string true "Discord guild ID"
// @Param appealID path string true "Appeal ID"
// @Security CookieAuth
// @Success 200 {object} map[string]interface{}
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Router /guilds/{discordGuildID}/appeals/{appealID} [get]
func getStaffAppeal(c *gin.Context, appeals *quack.AppealService) {
	result, err := appeals.GetStaff(c.Request.Context(), middleware.GetGuildContext(c), c.Param("appealID"))
	if err != nil {
		writeAppealError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"appeal": result})
}

// requestAppealInformation asks the member for more appeal context.
// @Summary Request more appeal information
// @Tags Appeals
// @Accept json
// @Produce json
// @Param discordGuildID path string true "Discord guild ID"
// @Param appealID path string true "Appeal ID"
// @Param Idempotency-Key header string true "Retry-safe request key"
// @Param decision body quack.AppealDecisionInput true "Decision reason"
// @Security CookieAuth
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Router /guilds/{discordGuildID}/appeals/{appealID}/request-information [post]
func requestAppealInformation(c *gin.Context, appeals *quack.AppealService) {
	transitionAppeal(c, appeals, "request-information")
}

// reopenAppeal returns an appeal to staff review.
// @Summary Reopen an appeal
// @Tags Appeals
// @Accept json
// @Produce json
// @Param discordGuildID path string true "Discord guild ID"
// @Param appealID path string true "Appeal ID"
// @Param Idempotency-Key header string true "Retry-safe request key"
// @Param decision body quack.AppealDecisionInput true "Decision reason"
// @Security CookieAuth
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Router /guilds/{discordGuildID}/appeals/{appealID}/reopen [post]
func reopenAppeal(c *gin.Context, appeals *quack.AppealService) {
	transitionAppeal(c, appeals, "reopen")
}

// acceptAppeal accepts an appeal for separate reversal review.
// @Summary Accept an appeal
// @Tags Appeals
// @Accept json
// @Produce json
// @Param discordGuildID path string true "Discord guild ID"
// @Param appealID path string true "Appeal ID"
// @Param Idempotency-Key header string true "Retry-safe request key"
// @Param decision body quack.AppealDecisionInput true "Decision reason"
// @Security CookieAuth
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Router /guilds/{discordGuildID}/appeals/{appealID}/accept [post]
func acceptAppeal(c *gin.Context, appeals *quack.AppealService) {
	transitionAppeal(c, appeals, "accept")
}

// rejectAppeal rejects an appeal with a staff reason.
// @Summary Reject an appeal
// @Tags Appeals
// @Accept json
// @Produce json
// @Param discordGuildID path string true "Discord guild ID"
// @Param appealID path string true "Appeal ID"
// @Param Idempotency-Key header string true "Retry-safe request key"
// @Param decision body quack.AppealDecisionInput true "Decision reason"
// @Security CookieAuth
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Router /guilds/{discordGuildID}/appeals/{appealID}/reject [post]
func rejectAppeal(c *gin.Context, appeals *quack.AppealService) {
	transitionAppeal(c, appeals, "reject")
}

// closeAppeal closes an appeal without changing the case.
// @Summary Close an appeal
// @Tags Appeals
// @Accept json
// @Produce json
// @Param discordGuildID path string true "Discord guild ID"
// @Param appealID path string true "Appeal ID"
// @Param Idempotency-Key header string true "Retry-safe request key"
// @Param decision body quack.AppealDecisionInput true "Decision reason"
// @Security CookieAuth
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Router /guilds/{discordGuildID}/appeals/{appealID}/close [post]
func closeAppeal(c *gin.Context, appeals *quack.AppealService) {
	transitionAppeal(c, appeals, "close")
}

// transitionAppeal applies one explicit staff appeal state transition.
func transitionAppeal(c *gin.Context, appeals *quack.AppealService, transition string) {
	var input quack.AppealDecisionInput
	if err := decodeStrictJSON(c, &input); err != nil {
		apierror.Write(c, http.StatusBadRequest, apierror.CodeValidation, "invalid appeal decision payload")
		return
	}
	ctx := middleware.GetGuildContext(c)
	var result *quack.AppealResponse
	var err error
	switch transition {
	case "request-information":
		result, err = appeals.RequestInformation(c.Request.Context(), ctx, c.Param("appealID"), input.Reason)
	case "reopen":
		result, err = appeals.Reopen(c.Request.Context(), ctx, c.Param("appealID"), input.Reason)
	case "accept":
		result, err = appeals.Accept(c.Request.Context(), ctx, c.Param("appealID"), input.Reason)
	case "reject":
		result, err = appeals.Reject(c.Request.Context(), ctx, c.Param("appealID"), input.Reason)
	case "close":
		result, err = appeals.Close(c.Request.Context(), ctx, c.Param("appealID"), input.Reason)
	default:
		err = quack.ErrAppealValidation
	}
	if err != nil {
		writeAppealError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"appeal": result})
}

type appealReversalRequest struct {
	OriginalExecutionID string           `json:"original_execution_id"`
	ActionType          model.ActionType `json:"action_type"`
	Confirm             bool             `json:"confirm"`
}

// reverseAcceptedAppeal queues a separately confirmed reversal for an accepted appeal.
// @Summary Reverse an accepted appeal action
// @Tags Appeals
// @Accept json
// @Produce json
// @Param discordGuildID path string true "Discord guild ID"
// @Param appealID path string true "Appeal ID"
// @Param Idempotency-Key header string true "Retry-safe request key"
// @Param reversal body appealReversalRequest true "Confirmed reversal"
// @Security CookieAuth
// @Success 202 {object} map[string]interface{}
// @Failure 400 {object} apierror.Response
// @Failure 403 {object} apierror.Response
// @Failure 404 {object} apierror.Response
// @Failure 409 {object} apierror.Response
// @Router /guilds/{discordGuildID}/appeals/{appealID}/reversals [post]
func reverseAcceptedAppeal(c *gin.Context, services *quack.Services, appeals *quack.AppealService) {
	var input appealReversalRequest
	if err := decodeStrictJSON(c, &input); err != nil || !input.Confirm {
		apierror.Write(c, http.StatusBadRequest, apierror.CodeValidation, "confirmed reversal payload is required")
		return
	}
	appeal, err := appeals.GetStaff(c.Request.Context(), middleware.GetGuildContext(c), c.Param("appealID"))
	if err != nil {
		writeAppealError(c, err)
		return
	}
	if appeal.Status != model.AppealStatusAccepted {
		writeAppealError(c, quack.ErrAppealConflict)
		return
	}
	appealID := appeal.ID
	result, err := services.Actions.ReverseForAppeal(c.Request.Context(), middleware.GetGuildContext(c), appeal.CaseID, input.OriginalExecutionID, input.ActionType, &appealID)
	if err != nil {
		writeCaseError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"action": result})
}

func memberReadLimit(services *quack.Services) httpplatform.RateLimit {
	return httpplatform.RateLimit{Maximum: services.Config.RateLimits.MemberRead.Maximum, Window: time.Duration(services.Config.RateLimits.MemberRead.WindowSeconds) * time.Second}
}

func memberWriteIdempotency(primitives httpplatform.Primitives, services *quack.Services, class string) gin.HandlerFunc {
	return primitives.Idempotency.Protect(class, time.Duration(services.Config.RateLimits.IdempotencyTTLHours)*time.Hour, memberWriteSubject)
}

func staffWriteIdempotency(primitives httpplatform.Primitives, services *quack.Services, class string) gin.HandlerFunc {
	return primitives.Idempotency.Protect(class, time.Duration(services.Config.RateLimits.IdempotencyTTLHours)*time.Hour, staffWriteSubject)
}

func memberSubject(c *gin.Context) string {
	session := middleware.GetAuthSession(c)
	if session == nil {
		return "unknown"
	}
	return session.DiscordUserID
}

func memberWriteSubject(c *gin.Context) string {
	return memberSubject(c) + ":" + c.Request.Method + ":" + c.FullPath() + ":" + c.Param("caseID") + ":" + c.Param("appealID")
}

func staffAppealSubject(c *gin.Context) string {
	guildContext := middleware.GetGuildContext(c)
	if guildContext == nil || guildContext.Guild == nil {
		return "unknown"
	}
	return guildContext.Guild.ID + ":" + guildContext.ActorDiscordUserID
}

func staffWriteSubject(c *gin.Context) string {
	return staffAppealSubject(c) + ":" + c.Request.Method + ":" + c.FullPath() + ":" + c.Param("appealID")
}

func writeAppealError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, quack.ErrAppealValidation):
		apierror.Write(c, http.StatusBadRequest, apierror.CodeValidation, "invalid appeal request")
	case errors.Is(err, quack.ErrAppealPermissionDenied):
		apierror.Write(c, http.StatusForbidden, apierror.CodeAuthorization, "appeal access denied")
	case errors.Is(err, quack.ErrAppealNotFound), errors.Is(err, model.ErrAppealCaseIneligible):
		apierror.Write(c, http.StatusNotFound, apierror.CodeNotFound, "appeal not found")
	case errors.Is(err, quack.ErrAppealConflict):
		apierror.Write(c, http.StatusConflict, apierror.CodeConflict, "appeal state conflict")
	default:
		apierror.Write(c, http.StatusInternalServerError, apierror.CodeInternal, "appeal operation failed")
	}
}
