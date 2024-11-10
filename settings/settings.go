package settings

import (
	"bigbucks/solution/auth/loging"
	"bigbucks/solution/auth/models"
	"crypto/rand"
	"os"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v4"
	"gorm.io/gorm"
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

var (
	Current      *Settings
	keyReadSync  sync.Once
	SingingKey   []byte
	VerifyingKey []byte
)

type UserOrgRole struct {
	Role  string `json:"role"`
	OrgID int    `json:"orgID"`
}

type UserInfo struct {
	Username string        `json:"username"`
	Roles    []UserOrgRole `json:"roles"`
}

type AuthToken struct {
	User UserInfo `json:"user"`
	jwt.RegisteredClaims
}

// Context :: Http Context Object
type Context struct {
	Auth     AuthToken `json:"user"`
	Settings Settings  `json:"settings"`
}

// Server specific settings.
type Settings struct {
	SecretKey  string `json:"key" mapstructure:"key"`
	BaseURL    string `json:"baseURL"`
	Port       string `json:"port"`
	Address    string `json:"address"`
	Log        string `json:"log"`
	Alg        string `json:"alg"`
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	DBUsername string `json:"dBUsername"`
	DBPassword string `json:"dBPassword"`
	DBName     string `json:"dBName"`
	DBHost     string `json:"dBHost"`
	DBPort     string `json:"dBPort"`
	DBSSLMode  string `json:"dBSSLMode"`
}

// Clean cleans any variables that might need cleaning.
func (s *Settings) Clean() {
	s.BaseURL = strings.TrimSuffix(s.BaseURL, "/")
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// LoadKeys Loads the pem key from file to memory
func (s *Settings) LoadKeys() {
	keyReadSync.Do(func() {
		dat, err := os.ReadFile(s.PrivateKey)
		check(err)
		SingingKey = dat
		pubdat, err := os.ReadFile(s.PublicKey)
		check(err)
		VerifyingKey = pubdat
	})
}

func (c *Context) GetCurrentUserModel() (user *models.User, err error) {
	if err := models.Dbcon.Where("username = ?", c.Auth.User.Username).First(&user).Error; gorm.ErrRecordNotFound == err {
		loging.Logger.Debugln(err)
		return nil, err
	}
	return
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
