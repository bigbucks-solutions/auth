package settings

import (
	"crypto/rand"
	"strings"
	"bigbucks/solution/auth/models"
)

// AuthMethod describes an authentication method.
// type AuthMethod string

// // Settings contain the main settings of the application.
// type Settings struct {
// 	Key           []byte              `json:"key"`
// 	Signup        bool                `json:"signup"`
// 	CreateUserDir bool                `json:"createUserDir"`
// 	Defaults      UserDefaults        `json:"defaults"`
// 	AuthMethod    AuthMethod          `json:"authMethod"`
// 	Branding      Branding            `json:"branding"`
// 	Commands      map[string][]string `json:"commands"`
// 	Shell         []string            `json:"shell"`
// 	Rules         []rules.Rule        `json:"rules"`
// }

// // GetRules implements rules.Provider.
// func (s *Settings) GetRules() []rules.Rule {
// 	return s.Rules
// }

// Context :: Http Context Object
type Context struct{
	User models.User `json:"user"`
	Settings Settings `json:"settings"`
}

// Server specific settings.
type Settings struct {
	SecretKey string `json:"key" mapstructure:"key"`
	BaseURL   string `json:"baseURL"`
	Port      string `json:"port"`
	Address   string `json:"address"`
	Log       string `json:"log"`
}

// Clean cleans any variables that might need cleaning.
func (s *Settings) Clean() {
	s.BaseURL = strings.TrimSuffix(s.BaseURL, "/")
}

// GenerateKey generates a key of 256 bits.
func GenerateKey() ([]byte, error) {
	b := make([]byte, 64)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}
