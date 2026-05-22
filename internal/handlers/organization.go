package handlers

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/banglab2bb2c/banglab2bb2c/internal/audit"
	"github.com/banglab2bb2c/banglab2bb2c/internal/models"
	"github.com/banglab2bb2c/banglab2bb2c/internal/utils"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// generalSettingsSnapshot extracts the fields shown on the General tab into a
// map suitable for audit diffing. Reading from a nil JSONB map returns the
// zero value (nil), which is treated as "unset" by the audit comparator.
func generalSettingsSnapshot(name string, settings models.JSONB) map[string]any {
	return map[string]any{
		"name":               name,
		"timezone":           settings["timezone"],
		"date_format":        settings["date_format"],
		"mask_phone_numbers": settings["mask_phone_numbers"],
	}
}

// OrganizationSettings represents the settings structure
type OrganizationSettings struct {
	MaskPhoneNumbers bool   `json:"mask_phone_numbers"`
	Timezone         string `json:"timezone"`
	DateFormat       string `json:"date_format"`
}

// GetOrganizationSettings returns the organization settings
func (a *App) GetOrganizationSettings(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Organization not found", nil, "")
	}

	// Parse settings from JSONB
	settings := OrganizationSettings{
		MaskPhoneNumbers: false,
		Timezone:         "UTC",
		DateFormat:       "YYYY-MM-DD",
	}

	if org.Settings != nil {
		if v, ok := org.Settings["mask_phone_numbers"].(bool); ok {
			settings.MaskPhoneNumbers = v
		}
		if v, ok := org.Settings["timezone"].(string); ok && v != "" {
			settings.Timezone = v
		}
		if v, ok := org.Settings["date_format"].(string); ok && v != "" {
			settings.DateFormat = v
		}
	}

	return r.SendEnvelope(map[string]any{
		"settings": settings,
		"name":     org.Name,
	})
}

// UpdateOrganizationSettings updates the organization settings
func (a *App) UpdateOrganizationSettings(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req struct {
		MaskPhoneNumbers *bool   `json:"mask_phone_numbers"`
		Timezone         *string `json:"timezone"`
		DateFormat       *string `json:"date_format"`
		Name             *string `json:"name"`
	}

	if err := json.Unmarshal(r.RequestCtx.PostBody(), &req); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Organization not found", nil, "")
	}

	// Snapshot before mutation so we can compute a diff.
	oldGeneral := generalSettingsSnapshot(org.Name, org.Settings)

	generalTouched := req.MaskPhoneNumbers != nil || req.Timezone != nil || req.DateFormat != nil || (req.Name != nil && *req.Name != "")

	// Update settings
	if org.Settings == nil {
		org.Settings = models.JSONB{}
	}

	if req.MaskPhoneNumbers != nil {
		org.Settings["mask_phone_numbers"] = *req.MaskPhoneNumbers
	}
	if req.Timezone != nil {
		org.Settings["timezone"] = *req.Timezone
	}
	if req.DateFormat != nil {
		org.Settings["date_format"] = *req.DateFormat
	}
	if req.Name != nil && *req.Name != "" {
		org.Name = *req.Name
	}

	if err := a.DB.Save(&org).Error; err != nil {
		a.Log.Error("Failed to update settings", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update settings", nil, "")
	}

	if generalTouched {
		userName := audit.GetUserName(a.DB, userID)
		newGeneral := generalSettingsSnapshot(org.Name, org.Settings)
		audit.LogAudit(a.DB, orgID, userID, userName,
			models.ResourceSettingsGeneral, orgID, models.AuditActionUpdated, oldGeneral, newGeneral)
	}

	return r.SendEnvelope(map[string]any{
		"message": "Settings updated successfully",
	})
}

// MaskContactFields conditionally masks a profile name and phone number
// if phone masking is enabled for the given organization.
func (a *App) MaskContactFields(orgID any, profileName, phoneNumber string) (string, string) {
	if a.ShouldMaskPhoneNumbers(orgID) {
		return utils.MaskIfPhoneNumber(profileName), utils.MaskPhoneNumber(phoneNumber)
	}
	return profileName, phoneNumber
}

// ShouldMaskPhoneNumbers checks if phone masking is enabled for the organization
func (a *App) ShouldMaskPhoneNumbers(orgID any) bool {
	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return false
	}

	if org.Settings != nil {
		if v, ok := org.Settings["mask_phone_numbers"].(bool); ok {
			return v
		}
	}
	return false
}

// OrganizationResponse represents an organization in API responses
type OrganizationResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug,omitempty"`
	CreatedAt string    `json:"created_at"`
}

// ListOrganizations returns all organizations (super admin or users with organizations:read)
func (a *App) ListOrganizations(r *fastglue.Request) error {
	userID, ok := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Super admins or users with organizations:read permission
	if !a.IsSuperAdmin(userID) && !a.HasPermission(userID, models.ResourceOrganizations, models.ActionRead) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Insufficient permissions", nil, "")
	}

	var orgs []models.Organization
	if err := a.DB.Order("name ASC").Find(&orgs).Error; err != nil {
		a.Log.Error("Failed to list organizations", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list organizations", nil, "")
	}

	response := make([]OrganizationResponse, len(orgs))
	for i, org := range orgs {
		response[i] = OrganizationResponse{
			ID:        org.ID,
			Name:      org.Name,
			Slug:      org.Slug,
			CreatedAt: org.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	return r.SendEnvelope(map[string]any{
		"organizations": response,
	})
}

// GetCurrentOrganization returns the current user's organization details
func (a *App) GetCurrentOrganization(r *fastglue.Request) error {
	orgID, err := a.getOrgID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var org models.Organization
	if err := a.DB.Where("id = ?", orgID).First(&org).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Organization not found", nil, "")
	}

	return r.SendEnvelope(OrganizationResponse{
		ID:        org.ID,
		Name:      org.Name,
		Slug:      org.Slug,
		CreatedAt: org.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// CreateOrganizationRequest represents the request body for creating an organization
type CreateOrganizationRequest struct {
	Name string `json:"name"`
}

// CreateOrganization is disabled — this is a single-tenant build with one
// seeded organization (BANGLAB2BB2C). The route is left registered so older
// clients get an explicit 403 instead of silently succeeding.
func (a *App) CreateOrganization(r *fastglue.Request) error {
	return r.SendErrorEnvelope(fasthttp.StatusForbidden,
		"This deployment runs in single-tenant mode; creating additional organizations is disabled.", nil, "")
}

// MemberResponse represents an organization member in API responses
type MemberResponse struct {
	ID             uuid.UUID  `json:"id"`
	UserID         uuid.UUID  `json:"user_id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	RoleID         *uuid.UUID `json:"role_id,omitempty"`
	RoleName       string     `json:"role_name,omitempty"`
	IsDefault      bool       `json:"is_default"`
	Email          string     `json:"email"`
	FullName       string     `json:"full_name"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
}

// ListOrganizationMembers returns all members of the current organization
func (a *App) ListOrganizationMembers(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceOrganizations, models.ActionRead); err != nil {
		return nil
	}

	pg := parsePagination(r)
	search := string(r.RequestCtx.QueryArgs().Peek("search"))

	baseQuery := a.DB.Table("user_organizations").
		Joins("LEFT JOIN users ON users.id = user_organizations.user_id AND users.deleted_at IS NULL").
		Joins("LEFT JOIN custom_roles ON custom_roles.id = user_organizations.role_id AND custom_roles.deleted_at IS NULL").
		Where("user_organizations.organization_id = ? AND user_organizations.deleted_at IS NULL", orgID)

	if search != "" {
		baseQuery = baseQuery.Where("users.full_name ILIKE ? OR users.email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	baseQuery.Count(&total)

	var response []MemberResponse
	if err := pg.Apply(baseQuery.
		Select(`user_organizations.id, user_organizations.user_id, user_organizations.organization_id,
			user_organizations.role_id, user_organizations.is_default, user_organizations.created_at,
			users.email, users.full_name, users.is_active,
			custom_roles.name AS role_name`).
		Order("user_organizations.created_at DESC")).
		Scan(&response).Error; err != nil {
		a.Log.Error("Failed to list organization members", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list members", nil, "")
	}

	return r.SendEnvelope(map[string]any{
		"members": response,
		"total":   total,
		"page":    pg.Page,
		"limit":   pg.Limit,
	})
}

// AddMemberRequest represents the request body for adding a member to an organization
type AddMemberRequest struct {
	UserID uuid.UUID  `json:"user_id"`
	Email  string     `json:"email"`
	RoleID *uuid.UUID `json:"role_id"`
}

// AddOrganizationMember adds an existing user to the current organization
func (a *App) AddOrganizationMember(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceOrganizations, models.ActionAssign); err != nil {
		return nil
	}

	var req AddMemberRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	// Resolve target user by user_id or email
	var targetUser models.User
	if req.UserID != uuid.Nil {
		if err := a.DB.Where("id = ?", req.UserID).First(&targetUser).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
		}
	} else if req.Email != "" {
		if err := a.DB.Where("email = ?", req.Email).First(&targetUser).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusNotFound, "No user found with this email", nil, "")
		}
	} else {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "user_id or email is required", nil, "")
	}

	// Check if already a member
	var existingCount int64
	a.DB.Model(&models.UserOrganization{}).
		Where("user_id = ? AND organization_id = ?", targetUser.ID, orgID).
		Count(&existingCount)
	if existingCount > 0 {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "User is already a member of this organization", nil, "")
	}

	// Determine role
	var roleID *uuid.UUID
	if req.RoleID != nil {
		// Validate role exists and belongs to org
		var role models.CustomRole
		if err := a.DB.Where("id = ? AND organization_id = ?", req.RoleID, orgID).First(&role).Error; err != nil {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid role", nil, "")
		}
		roleID = req.RoleID
	} else {
		// Use org's default role
		var defaultRole models.CustomRole
		if err := a.DB.Where("organization_id = ? AND is_default = ?", orgID, true).First(&defaultRole).Error; err == nil {
			roleID = &defaultRole.ID
		}
	}

	userOrg := models.UserOrganization{
		UserID:         targetUser.ID,
		OrganizationID: orgID,
		RoleID:         roleID,
		IsDefault:      false,
	}

	if err := a.DB.Create(&userOrg).Error; err != nil {
		a.Log.Error("Failed to add organization member", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to add member", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "Member added successfully"})
}

// RemoveOrganizationMember removes a user from the current organization
func (a *App) RemoveOrganizationMember(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceOrganizations, models.ActionAssign); err != nil {
		return nil
	}

	targetUserID, err := parsePathUUID(r, "member_id", "member")
	if err != nil {
		return nil
	}

	// Cannot remove self
	if targetUserID == userID {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot remove yourself from the organization", nil, "")
	}

	result := a.DB.Where("user_id = ? AND organization_id = ?", targetUserID, orgID).
		Delete(&models.UserOrganization{})
	if result.Error != nil {
		a.Log.Error("Failed to remove organization member", "error", result.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to remove member", nil, "")
	}
	if result.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Member not found in this organization", nil, "")
	}

	// Invalidate removed user's permission cache
	a.InvalidateUserPermissionsCache(targetUserID)

	return r.SendEnvelope(map[string]string{"message": "Member removed successfully"})
}

// UpdateMemberRoleRequest represents the request body for updating a member's role
type UpdateMemberRoleRequest struct {
	RoleID uuid.UUID `json:"role_id"`
}

// UpdateOrganizationMemberRole updates a member's role in the current organization
func (a *App) UpdateOrganizationMemberRole(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	if err := a.requirePermission(r, userID, models.ResourceOrganizations, models.ActionAssign); err != nil {
		return nil
	}

	targetUserID, err := parsePathUUID(r, "member_id", "member")
	if err != nil {
		return nil
	}

	var req UpdateMemberRoleRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.RoleID == uuid.Nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "role_id is required", nil, "")
	}

	// Validate role exists and belongs to org
	var role models.CustomRole
	if err := a.DB.Where("id = ? AND organization_id = ?", req.RoleID, orgID).First(&role).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid role", nil, "")
	}

	// Update the user's role in this org
	result := a.DB.Model(&models.UserOrganization{}).
		Where("user_id = ? AND organization_id = ?", targetUserID, orgID).
		Update("role_id", req.RoleID)
	if result.Error != nil {
		a.Log.Error("Failed to update member role", "error", result.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update member role", nil, "")
	}
	if result.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Member not found in this organization", nil, "")
	}

	// Invalidate permission cache
	a.InvalidateUserPermissionsCache(targetUserID)

	return r.SendEnvelope(map[string]string{"message": "Member role updated successfully"})
}
