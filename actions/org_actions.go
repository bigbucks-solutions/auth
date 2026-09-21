package actions

import (
	"bigbucks/solution/auth/constants"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	"bigbucks/solution/auth/settings"
	valids "bigbucks/solution/auth/validations"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Organization struct {
	Name               string  `json:"name" validate:"required,min=4"`
	ContactEmail       string  `json:"email" validate:"required,email"`
	ContactNumber      string  `json:"phone" validate:"omitempty,valid_phone,min=5"`
	Address            string  `json:"address"`
	City               string  `json:"city"`
	PostalCode         string  `json:"postal_code"`
	State              string  `json:"state"`
	Country            string  `json:"country" validate:"omitempty,iso3166_1_alpha2"`
	Currency           string  `json:"currency" validate:"omitempty,iso4217"`
	Latitude           float64 `json:"latitude"`
	Longitude          float64 `json:"longitude"`
	LogoURL            string  `json:"logo_url"`
	TaxID              string  `json:"tax_id"`
	WebsiteURL         string  `json:"website" validate:"omitempty,url"`
	CompanyDescription string  `json:"description" validate:"omitempty,max=500"`
}

// errCountryImmutable is returned when an update tries to move an
// organization to a different country. Subscription plans are priced per
// country (see subscriptions.CurrencyConfig), so the country an organization
// is billed under is fixed at creation; changing it is a billing operation,
// not a settings edit.
var errCountryImmutable = errors.New(
	"organization country cannot be changed because plans are priced per country; " +
		"contact support to move the organization",
)

// allowedLogoExts lists image extensions accepted for logo file uploads.
var allowedLogoExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".svg": true,
}

// OrganizationFromRequest parses an Organization from an HTTP request.
// It handles both multipart/form-data (with optional logo file upload) and JSON bodies.
func OrganizationFromRequest(r *http.Request) (*Organization, int, error) {
	var org Organization
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			return nil, http.StatusBadRequest, err
		}
		org.Name = r.FormValue("name")
		org.ContactEmail = r.FormValue("email")
		org.ContactNumber = r.FormValue("phone")
		org.Address = r.FormValue("address")
		org.City = r.FormValue("city")
		org.PostalCode = r.FormValue("postal_code")
		org.State = r.FormValue("state")
		org.Country = r.FormValue("country")
		org.Currency = r.FormValue("currency")
		org.WebsiteURL = r.FormValue("website")
		org.CompanyDescription = r.FormValue("description")
		org.LogoURL = r.FormValue("logo_url")
		org.TaxID = r.FormValue("tax_id")
		if latStr := r.FormValue("latitude"); latStr != "" {
			lat, err := strconv.ParseFloat(latStr, 64)
			if err != nil {
				return nil, http.StatusBadRequest, errors.New("invalid latitude")
			}
			org.Latitude = lat
		}
		if lngStr := r.FormValue("longitude"); lngStr != "" {
			lng, err := strconv.ParseFloat(lngStr, 64)
			if err != nil {
				return nil, http.StatusBadRequest, errors.New("invalid longitude")
			}
			org.Longitude = lng
		}
		// Handle optional logo file upload.
		file, header, err := r.FormFile("logo")
		if err == nil {
			defer func() { _ = file.Close() }()
			ext := strings.ToLower(filepath.Ext(header.Filename))
			if !allowedLogoExts[ext] {
				return nil, http.StatusBadRequest, errors.New("unsupported logo file type; allowed: jpg, jpeg, png, gif, webp, svg")
			}
			if err := os.MkdirAll("./org_logos", os.ModePerm); err != nil {
				return nil, http.StatusInternalServerError, err
			}
			filename := strings.ReplaceAll(uuid.New().String(), "-", "") + ext
			dst, err := os.Create("./org_logos/" + filename)
			if err != nil {
				return nil, http.StatusInternalServerError, err
			}
			defer func() { _ = dst.Close() }()
			if _, err := io.Copy(dst, file); err != nil {
				return nil, http.StatusInternalServerError, err
			}
			org.LogoURL = "/org-logo/" + filename
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&org); err != nil {
			return nil, http.StatusBadRequest, err
		}
	}
	// The ISO-3166 validator matches upper-case codes exactly, and the currency
	// mapping looks the stored value up the same way, so normalise once here
	// rather than letting "ae" fail validation or "  AE " miss the map.
	org.Country = strings.ToUpper(strings.TrimSpace(org.Country))
	// iso4217 matches upper-case codes exactly, same as the country validator.
	org.Currency = strings.ToUpper(strings.TrimSpace(org.Currency))
	return &org, 0, nil
}

// CreateOrganization : Create new Organization with a super user attached
func CreateOrganisationFromAuthenticatedUser(org *Organization, userName string, perm_cache *permission_cache.PermissionCache, ctx context.Context) (*models.OrganizationDetails, int, error) {
	err := valids.Validate.Struct(org)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	var orgModel models.Organization
	orgModel.Name = org.Name
	orgModel.Address = org.Address
	orgModel.City = org.City
	orgModel.PostalCode = org.PostalCode
	orgModel.State = org.State
	orgModel.ContactEmail = org.ContactEmail
	orgModel.ContactNumber = org.ContactNumber
	orgModel.Country = org.Country
	orgModel.Currency = org.Currency
	orgModel.Latitude = org.Latitude
	orgModel.Longitude = org.Longitude
	orgModel.LogoURL = org.LogoURL
	orgModel.TaxID = org.TaxID
	orgModel.WebsiteURL = org.WebsiteURL
	orgModel.CompanyDescription = org.CompanyDescription
	var ownerRole models.Role
	err = models.Dbcon.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Users").Create(&orgModel).Error; err != nil {
			return err
		}

		// Create Owner role per organization
		ownerRole = models.Role{
			Name:         "Owner",
			IsSystemRole: true,
			OrgID:        orgModel.ID,
		}
		if err := tx.Create(&ownerRole).Error; err != nil {
			return err
		}
		// Link user to organization with super admin role
		var userID string
		err := tx.Model(&models.User{}).Where("username = ?", userName).Select("id").Take(&userID).Error
		if err != nil {
			return err
		}

		if err := tx.Create(&models.UserOrgRole{OrgID: orgModel.ID,
			UserID: userID,
			RoleID: ownerRole.ID}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, http.StatusConflict, err
	}
	for _, resource := range ownerPermissionResources(settings.Current.ExtraPermResources) {
		if err := AssignSystemPermissionToRole(ownerRole.ID, orgModel.ID, resource, string(constants.ScopeAll), string(constants.ActionWrite), false, perm_cache, ctx); err != nil {
			return nil, http.StatusConflict, err
		}
	}

	details := orgModel.Details()
	return &details, 0, nil
}

func ownerPermissionResources(extraResources []string) []string {
	resources := make([]string, 0, len(constants.Resources)+len(extraResources))
	seen := make(map[string]struct{}, cap(resources))
	addResource := func(resource string) {
		resource = strings.ToLower(strings.TrimSpace(resource))
		if resource == "" {
			return
		}
		if _, exists := seen[resource]; exists {
			return
		}
		seen[resource] = struct{}{}
		resources = append(resources, resource)
	}
	for _, resource := range constants.Resources {
		addResource(resource)
	}
	for _, resource := range extraResources {
		addResource(resource)
	}
	return resources
}

// countryMoveRefused reports whether an update would move an organization to a
// different country, which is not allowed: plans are priced per country, so the
// country an organization is billed under is fixed once it has one.
//
// Both operands are already upper-cased and trimmed by OrganizationFromRequest.
// Two cases are deliberately *not* a move:
//
//   - a blank request value — the settings form PUTs the whole record, and a
//     client that omits the field is not asking for anything;
//   - a blank stored value — an organization created before the field was
//     collected can still have it filled in.
func countryMoveRefused(existing, requested string) bool {
	return existing != "" && requested != "" && requested != existing
}

// UpdateOrganization replaces the organization's editable details.
//
// Only the organization's Owner may change settings. The whole record is
// supplied (the request shares CreateOrg's validation, so name and contact
// email stay required); membership and role links are never touched here.
//
// Country is deliberately not editable — see errCountryImmutable.
func UpdateOrganization(orgID string, org *Organization, userName string) (*models.OrganizationDetails, int, error) {
	existing, err := models.GetOrganization(orgID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, http.StatusNotFound, errors.New("organization not found")
		}
		return nil, http.StatusInternalServerError, err
	}
	isOwner, err := models.IsOrganizationOwner(orgID, userName)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	if !isOwner {
		return nil, http.StatusForbidden, errors.New("only the organization owner can change its settings")
	}
	if err := valids.Validate.Struct(org); err != nil {
		return nil, http.StatusBadRequest, err
	}
	if countryMoveRefused(existing.Country, org.Country) {
		return nil, http.StatusConflict, errCountryImmutable
	}
	fields := map[string]any{
		"name":                org.Name,
		"contact_email":       org.ContactEmail,
		"contact_number":      org.ContactNumber,
		"address":             org.Address,
		"city":                org.City,
		"postal_code":         org.PostalCode,
		"state":               org.State,
		"currency":            org.Currency,
		"latitude":            org.Latitude,
		"longitude":           org.Longitude,
		"tax_id":              org.TaxID,
		"website_url":         org.WebsiteURL,
		"company_description": org.CompanyDescription,
	}
	// Written only when it was blank — countryMoveRefused rejects a change, so
	// this can fill the field in but never move it.
	if existing.Country == "" && org.Country != "" {
		fields["country"] = org.Country
	}
	// A logo is only replaced when a new one was uploaded or a URL supplied.
	if org.LogoURL != "" {
		fields["logo_url"] = org.LogoURL
	}
	if err := models.UpdateOrganizationFields(orgID, fields); err != nil {
		return nil, http.StatusConflict, err
	}
	updated, err := models.GetOrganization(orgID)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	details := updated.Details()
	return &details, 0, nil
}
