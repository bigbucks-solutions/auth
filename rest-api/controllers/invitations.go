package controllers

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/request_context"
	"bigbucks/solution/auth/rest-api/controllers/types"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// InviteUserRequest represents the request body for inviting a user
type InviteUserRequest struct {
	Email  string `json:"email" validate:"required,email"`
	RoleID string `json:"roleId" validate:"required"`
}

// AcceptInvitationRequest represents the request body for accepting an invitation
type AcceptInvitationRequest struct {
	Token string `json:"token" validate:"required"`
}

// InvitationResponse represents the response format for invitations
type InvitationResponse struct {
	ID         string   `json:"id"`
	Email      string   `json:"email"`
	Status     string   `json:"status"`
	Role       RoleInfo `json:"role"`
	Inviter    UserInfo `json:"inviter"`
	CreatedAt  string   `json:"createdAt"`
	ExpiresAt  string   `json:"expiresAt"`
	AcceptedAt *string  `json:"acceptedAt,omitempty"`
}

type RoleInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type ListInvitationsResponse struct {
	Invitations []InvitationResponse `json:"invitations"`
	Total       int64                `json:"total"`
	Page        int                  `json:"page"`
	Size        int                  `json:"size"`
}

// @Summary		Invite user to organization
// @Description	Send an invitation to a user to join the organization with a specific role
// @Tags			invitations
// @Accept			json
// @Produce		json
// @Param			X-Auth	header	string	true	"Authorization"
// @Param			invitation	body	InviteUserRequest	true	"Invitation details"
// @Success		201		{object}	InvitationResponse	"Invitation sent successfully"
// @Failure		400		{object}	error				"Bad request"
// @Failure		401		{object}	error				"Unauthorized"
// @Failure		500		{object}	error				"Internal server error"
// @Router			/invitations [post]
func InviteUser(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	var req InviteUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		loging.Logger.Error("Failed to decode invitation request", err)
		return http.StatusBadRequest, err
	}

	// Get current user from context
	currentUser, err := ctx.GetCurrentUserModel()
	if err != nil {
		loging.Logger.Error("Failed to get current user", err)
		return http.StatusUnauthorized, err
	}

	params := actions.InviteUserParams{
		Email:     req.Email,
		OrgID:     ctx.CurrentOrgID,
		RoleID:    req.RoleID,
		InviterID: currentUser.ID,
	}

	invitation, code, err := actions.InviteUserToOrg(params)
	if err != nil {
		return code, err
	}

	response := InvitationResponse{
		ID:     invitation.ID,
		Email:  invitation.Email,
		Status: string(invitation.Status),
		Role: RoleInfo{
			ID:   invitation.Role.ID,
			Name: invitation.Role.Name,
		},
		Inviter: UserInfo{
			ID:       invitation.Inviter.ID,
			Username: invitation.Inviter.Username,
		},
		CreatedAt: invitation.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		ExpiresAt: invitation.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if invitation.AcceptedAt != nil {
		acceptedAt := invitation.AcceptedAt.Format("2006-01-02T15:04:05Z07:00")
		response.AcceptedAt = &acceptedAt
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	return 0, json.NewEncoder(w).Encode(response)
}

// @Summary		List invitations for organization
// @Description	Get paginated list of invitations for the current organization with sorting support
// @Tags			invitations
// @Accept			json
// @Produce		json
// @Param			X-Auth	header	string	true	"Authorization"
// @Param			page		query		int		false	"Page number"	default(1)
// @Param			page_size	query		int		false	"Page size"		default(10)
// @Param			status		query		string	false	"Filter by status (pending, accepted, expired, revoked)"
// @Param			order_by	query		string	false	"Order by field (email, status, created_at, expires_at, role_name, inviter_name)"	default(created_at)
// @Param			order_dir	query		string	false	"Order direction (asc, desc)"	default(desc)
// @Param			search		query		string	false	"Search term to filter by inviter or invitee email"
// @Success		200		{object}	ListInvitationsResponse
// @Failure		400		{object}	error					"Bad request"
// @Failure		401		{object}	error					"Unauthorized"
// @Failure		500		{object}	error					"Internal server error"
// @Router			/invitations [get]
func ListInvitations(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 {
		pageSize = 10
	}

	statusParam := r.URL.Query().Get("status")
	var status *models.InvitationStatus
	if statusParam != "" {
		s := models.InvitationStatus(statusParam)
		status = &s
	}

	// Get ordering parameters
	orderBy := r.URL.Query().Get("order_by")
	orderDir := r.URL.Query().Get("order_dir")

	// Get search parameter
	search := r.URL.Query().Get("search")

	// Build parameters struct
	params := actions.ListInvitationsParams{
		OrgID:    ctx.CurrentOrgID,
		Page:     page,
		PageSize: pageSize,
		Status:   status,
		OrderBy:  orderBy,
		OrderDir: orderDir,
		Search:   search,
	}

	invitations, total, code, err := actions.ListInvitations(params)
	if err != nil {
		return code, err
	}

	// Convert to response format
	invitationResponses := make([]InvitationResponse, len(invitations))
	for i, inv := range invitations {
		invitationResponses[i] = InvitationResponse{
			ID:     inv.ID,
			Email:  inv.Email,
			Status: string(inv.Status),
			Role: RoleInfo{
				ID:   inv.Role.ID,
				Name: inv.Role.Name,
			},
			Inviter: UserInfo{
				ID:       inv.Inviter.ID,
				Username: inv.Inviter.Username,
			},
			CreatedAt: inv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			ExpiresAt: inv.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		if inv.AcceptedAt != nil {
			acceptedAt := inv.AcceptedAt.Format("2006-01-02T15:04:05Z07:00")
			invitationResponses[i].AcceptedAt = &acceptedAt
		}
	}

	response := ListInvitationsResponse{
		Invitations: invitationResponses,
		Total:       total,
		Page:        page,
		Size:        pageSize,
	}

	w.Header().Set("Content-Type", "application/json")
	return 0, json.NewEncoder(w).Encode(response)
}

// @Summary		Accept invitation
// @Description	Accept an invitation to join an organization
// @Tags			invitations
// @Accept			json
// @Produce		json
// @Param			X-Auth	header	string	true	"Authorization"
// @Param			token		query		string		true	"Invitation Token"
// @Success		200		{object}	types.SimpleResponse	"Invitation accepted successfully"
// @Failure		400		{object}	error					"Bad request"
// @Failure		401		{object}	error					"Unauthorized"
// @Failure		500		{object}	error					"Internal server error"
// @Router			/invitations/accept [get]
func AcceptInvitation(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	token := r.URL.Query().Get("token")
	if token == "" {
		loging.Logger.Error("Missing token parameter", nil)
		return http.StatusBadRequest, nil
	}

	// Get current user from context
	currentUser, err := ctx.GetCurrentUserModel()
	if err != nil {
		loging.Logger.Error("Failed to get current user", err)
		return http.StatusUnauthorized, err
	}

	code, err := actions.AcceptInvitation(token, currentUser.ID)
	if err != nil {
		return code, err
	}

	w.Header().Set("Content-Type", "application/json")
	return 0, json.NewEncoder(w).Encode(&types.SimpleResponse{
		Message: "Invitation accepted successfully",
	})
}

// @Summary		Revoke invitation
// @Description	Revoke a pending invitation
// @Tags			invitations
// @Accept			json
// @Produce		json
// @Param			X-Auth	header	string	true	"Authorization"
// @Param			invitation_id	path	string	true	"Invitation ID"
// @Success		200		{object}	types.SimpleResponse	"Invitation revoked successfully"
// @Failure		400		{object}	error					"Bad request"
// @Failure		401		{object}	error					"Unauthorized"
// @Failure		404		{object}	error					"Invitation not found"
// @Failure		500		{object}	error					"Internal server error"
// @Router			/invitations/{invitation_id}/revoke [put]
func RevokeInvitation(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	vars := mux.Vars(r)
	invitationID := vars["invitation_id"]

	if invitationID == "" {
		return http.StatusBadRequest, nil
	}

	code, err := actions.RevokeInvitation(invitationID, ctx.CurrentOrgID)
	if err != nil {
		return code, err
	}

	w.Header().Set("Content-Type", "application/json")
	return 0, json.NewEncoder(w).Encode(&types.SimpleResponse{
		Message: "Invitation revoked successfully",
	})
}

// ResendInvitation godoc
//
//	@Summary		Resend invitation
//	@Description	Resend an existing invitation or create a new one if expired
//	@Tags			invitations
//	@Accept			json
//	@Produce		json
//	@Param			X-Auth			header	string				true	"Authorization"
//	@Param			invitation_id	path	string				true	"Invitation ID"
//	@Success		200				{object}	InvitationResponse	"Invitation resent"
//	@Failure		400				{object}	error				"Bad request"
//	@Failure		404				{object}	error				"Not found"
//	@Failure		500				{object}	error				"Internal server error"
//	@Router			/invitations/{invitation_id}/resend [post]
func ResendInvitation(w http.ResponseWriter, r *http.Request, ctx *request_context.Context) (int, error) {
	vars := mux.Vars(r)
	invitationID := vars["invitation_id"]

	currentUser, err := ctx.GetCurrentUserModel()
	if err != nil {
		return http.StatusUnauthorized, err
	}

	invitation, code, err := actions.ResendInvitation(invitationID, ctx.CurrentOrgID, currentUser.ID)
	if err != nil {
		return code, err
	}

	response := InvitationResponse{
		ID:     invitation.ID,
		Email:  invitation.Email,
		Status: string(invitation.Status),
		Role: RoleInfo{
			ID:   invitation.Role.ID,
			Name: invitation.Role.Name,
		},
		Inviter: UserInfo{
			ID:       invitation.Inviter.ID,
			Username: invitation.Inviter.Username,
		},
		CreatedAt: invitation.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		ExpiresAt: invitation.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	w.Header().Set("Content-Type", "application/json")
	return 0, json.NewEncoder(w).Encode(response)
}
