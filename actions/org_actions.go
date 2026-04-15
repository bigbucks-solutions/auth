package actions

import (
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
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
	Country            string  `json:"country"`
	Latitude           float64 `json:"latitude"`
	Longitude          float64 `json:"longitude"`
	LogoURL            string  `json:"logo_url"`
	TaxID              string  `json:"tax_id"`
	WebsiteURL         string  `json:"website" validate:"omitempty,url"`
	CompanyDescription string  `json:"description" validate:"omitempty,max=500"`
}

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
	return &org, 0, nil
}

// CreateOrganization : Create new Organization with a super user attached
func CreateOrganisationFromAuthenticatedUser(org *Organization, userName string, perm_cache *permission_cache.PermissionCache, ctx context.Context) (int, error) {
	err := valids.Validate.Struct(org)
	if err != nil {
		return http.StatusBadRequest, err
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
	orgModel.Latitude = org.Latitude
	orgModel.Longitude = org.Longitude
	orgModel.LogoURL = org.LogoURL
	orgModel.TaxID = org.TaxID
	orgModel.WebsiteURL = org.WebsiteURL
	orgModel.CompanyDescription = org.CompanyDescription
	var SuperAdminRole models.Role
	err = models.Dbcon.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Users").Create(&orgModel).Error; err != nil {
			return err
		}

		// Create Admin role per organization
		SuperAdminRole = models.Role{
			Name:         "Admin",
			IsSystemRole: true,
			OrgID:        orgModel.ID,
		}
		if err := tx.Create(&SuperAdminRole).Error; err != nil {
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
			RoleID: SuperAdminRole.ID}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return http.StatusConflict, err
	}
	err = AssignSystemPermissionToRole(SuperAdminRole.ID, orgModel.ID, "session", "all", "write", false, perm_cache, ctx)
	if err != nil {
		return http.StatusConflict, err
	}
	err = AssignSystemPermissionToRole(SuperAdminRole.ID, orgModel.ID, "user", "all", "write", false, perm_cache, ctx)
	if err != nil {
		return http.StatusConflict, err
	}
	err = AssignSystemPermissionToRole(SuperAdminRole.ID, orgModel.ID, "role", "all", "write", false, perm_cache, ctx)
	if err != nil {
		return http.StatusConflict, err
	}
	err = AssignSystemPermissionToRole(SuperAdminRole.ID, orgModel.ID, "masterdata", "all", "write", false, perm_cache, ctx)
	if err != nil {
		return http.StatusConflict, err
	}

	return 0, nil
}
