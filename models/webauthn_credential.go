package models

import (
	"bigbucks/solution/auth/loging"

	"github.com/go-webauthn/webauthn/webauthn"
	"gorm.io/gorm"
)

// WebAuthnCredential stores a user's registered WebAuthn credential (passkey/security key).
type WebAuthnCredential struct {
	gorm.Model
	UserID          string `gorm:"type:char(26);index;not null"`
	Name            string `gorm:"type:varchar(255)"`               // Friendly name (e.g. "My YubiKey")
	CredentialID    []byte `gorm:"type:bytea;uniqueIndex;not null"` // Credential ID from authenticator
	PublicKey       []byte `gorm:"type:bytea;not null"`             // COSE public key
	AttestationType string `gorm:"type:varchar(64)"`                // Attestation type
	AAGUID          []byte `gorm:"type:bytea"`                      // Authenticator AAGUID
	SignCount       uint32 `gorm:"default:0"`                       // Signature counter
	Transport       string `gorm:"type:varchar(255);default:''"`    // Comma-separated transport hints
	Discoverable    bool   `gorm:"default:false"`                   // Whether credential supports resident key
	BackupEligible  bool   `gorm:"default:false"`                   // Whether credential supports multi-device sync
	BackupState     bool   `gorm:"default:false"`                   // Whether credential is currently backed up
}

// ToWebAuthnCredential converts the DB model to the go-webauthn Credential type.
func (wc *WebAuthnCredential) ToWebAuthnCredential() webauthn.Credential {
	return webauthn.Credential{
		ID:              wc.CredentialID,
		PublicKey:       wc.PublicKey,
		AttestationType: wc.AttestationType,
		Flags: webauthn.CredentialFlags{
			UserPresent:    true,
			UserVerified:   wc.Discoverable,
			BackupEligible: wc.BackupEligible,
			BackupState:    wc.BackupState,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:    wc.AAGUID,
			SignCount: wc.SignCount,
		},
	}
}

// GetWebAuthnCredentials retrieves all WebAuthn credentials for a user.
func GetWebAuthnCredentials(userID string) ([]WebAuthnCredential, error) {
	var creds []WebAuthnCredential
	if err := Dbcon.Where("user_id = ?", userID).Find(&creds).Error; err != nil {
		return nil, err
	}
	return creds, nil
}

// GetWebAuthnCredentialByCredentialID finds a credential by its WebAuthn credential ID bytes.
func GetWebAuthnCredentialByCredentialID(credentialID []byte) (*WebAuthnCredential, error) {
	var cred WebAuthnCredential
	if err := Dbcon.Where("credential_id = ?", credentialID).First(&cred).Error; err != nil {
		return nil, err
	}
	return &cred, nil
}

// SaveWebAuthnCredential persists a new WebAuthn credential to the database.
func SaveWebAuthnCredential(cred *WebAuthnCredential) error {
	return Dbcon.Create(cred).Error
}

// UpdateSignCount updates the signature counter for replay attack protection.
func UpdateSignCount(credentialID []byte, newCount uint32) error {
	return Dbcon.Model(&WebAuthnCredential{}).
		Where("credential_id = ?", credentialID).
		Update("sign_count", newCount).Error
}

// UpdateCredentialFlags updates the backup flags after a successful login.
func UpdateCredentialFlags(credentialID []byte, backupEligible, backupState bool) error {
	return Dbcon.Model(&WebAuthnCredential{}).
		Where("credential_id = ?", credentialID).
		Updates(map[string]interface{}{
			"backup_eligible": backupEligible,
			"backup_state":    backupState,
		}).Error
}

// DeleteWebAuthnCredential removes a credential by its DB ID and user ID.
func DeleteWebAuthnCredential(id uint, userID string) error {
	result := Dbcon.Where("id = ? AND user_id = ?", id, userID).Delete(&WebAuthnCredential{})
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return result.Error
}

// ListWebAuthnCredentials returns a summary of a user's registered credentials.
func ListWebAuthnCredentials(userID string) ([]WebAuthnCredential, error) {
	var creds []WebAuthnCredential
	if err := Dbcon.Select("id, name, created_at, discoverable, transport").
		Where("user_id = ?", userID).
		Find(&creds).Error; err != nil {
		return nil, err
	}
	return creds, nil
}

// WebAuthnUser wraps the User model to implement the webauthn.User interface.
type WebAuthnUser struct {
	User        User
	Credentials []webauthn.Credential
}

// WebAuthnID returns the user's unique ID as bytes.
func (u *WebAuthnUser) WebAuthnID() []byte {
	return []byte(u.User.ID)
}

// WebAuthnName returns the username (email).
func (u *WebAuthnUser) WebAuthnName() string {
	return u.User.Username
}

// WebAuthnDisplayName returns a human-readable display name.
func (u *WebAuthnUser) WebAuthnDisplayName() string {
	var profile Profile
	if err := Dbcon.Model(&u.User).Association("Profile").Find(&profile); err != nil {
		loging.Logger.Error("Error fetching profile for WebAuthn display name", err)
		return u.User.Username
	}
	if profile.FirstName != "" || profile.LastName != "" {
		return profile.FirstName + " " + profile.LastName
	}
	return u.User.Username
}

// WebAuthnCredentials returns the user's registered credentials.
func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return u.Credentials
}

// LoadWebAuthnUser loads a User and their WebAuthn credentials, returning a WebAuthnUser.
func LoadWebAuthnUser(userID string) (*WebAuthnUser, error) {
	var user User
	if err := Dbcon.Where("id = ?", userID).Preload("Roles").First(&user).Error; err != nil {
		return nil, err
	}
	return LoadWebAuthnUserFromModel(&user)
}

// LoadWebAuthnUserByUsername loads a WebAuthnUser by username.
func LoadWebAuthnUserByUsername(username string) (*WebAuthnUser, error) {
	var user User
	if err := Dbcon.Where("username = ?", username).Preload("Roles").First(&user).Error; err != nil {
		return nil, err
	}
	return LoadWebAuthnUserFromModel(&user)
}

// LoadWebAuthnUserFromModel wraps an existing User model with its credentials.
func LoadWebAuthnUserFromModel(user *User) (*WebAuthnUser, error) {
	dbCreds, err := GetWebAuthnCredentials(user.ID)
	if err != nil {
		return nil, err
	}
	var creds []webauthn.Credential
	for _, c := range dbCreds {
		creds = append(creds, c.ToWebAuthnCredential())
	}
	return &WebAuthnUser{
		User:        *user,
		Credentials: creds,
	}, nil
}
