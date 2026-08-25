package auth_test

import (
	"bigbucks/solution/auth/actions"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/permission_cache"
	router "bigbucks/solution/auth/rest-api"
	ctr "bigbucks/solution/auth/rest-api/controllers"
	sessionstore "bigbucks/solution/auth/session_store"
	"bigbucks/solution/auth/settings"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	valids "bigbucks/solution/auth/validations"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestDocker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auth Suite")
}

var (
	DB                 *gorm.DB
	cleanupDocker      func()
	s                  *httptest.Server
	c                  *http.Client
	TestUserID         string
	verificationEmails = &fakeVerificationSender{codes: make(map[string]string)}
)

type fakeVerificationSender struct {
	mu    sync.Mutex
	codes map[string]string
}

func (sender *fakeVerificationSender) SendEmail(string, string, any) error {
	return nil
}

func (sender *fakeVerificationSender) SendEmailWithSubject(email, _ string, _ string, params any) error {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	sender.codes[email] = params.(map[string]any)["Code"].(string)
	return nil
}

func (sender *fakeVerificationSender) Code(email string) string {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	return sender.codes[email]
}

var _ = BeforeSuite(func() {
	// setup *gorm.Db with docker
	models.Dbcon, cleanupDocker = setupGormWithDocker()
	models.Migrate()
	err := models.Dbcon.SetupJoinTable(&models.Organization{}, "Users", &models.UserOrgRole{})
	if err != nil {
		fmt.Println("Error setting up join table:", err)
	}
	err = models.Dbcon.SetupJoinTable(&models.User{}, "Roles", &models.UserOrgRole{})
	if err != nil {
		fmt.Println("Error setting up join table:", err)
	}
	err = models.Dbcon.SetupJoinTable(&models.Role{}, "Permissions", &models.RolePermission{})
	if err != nil {
		fmt.Println("Error setting up join table:", err)
	}

	GinkgoWriter.Println("Migration complete", err)

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		fmt.Println("Error generating private key:", err)
	}
	ecder, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		fmt.Println("Error marshaling private key:", err)
	}
	settings.SingingKey = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecder})
	x509EncodedPub, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	settings.VerifyingKey = pem.EncodeToMemory(&pem.Block{Type: "EC PUBLIC KEY", Bytes: x509EncodedPub})
	sampleData := &models.User{Username: "john@x.com", Password: "john123", EmailVerified: true, Profile: models.Profile{
		FirstName: "John", LastName: "Doe", Email: "john@x.com"},
	}
	err = models.Dbcon.Create(sampleData).Error
	Ω(err).To(Succeed())
	TestUserID = sampleData.ID
	settings.Current = &settings.Settings{Alg: "ES256", PrivateKey: "ec_private.pem", PublicKey: "ec_public.pem", LogLevel: "info", WebAuthnRPID: "localhost", WebAuthnRPName: "BigBucks Auth", EmailVerificationSecret: "test-email-verification-secret-32-bytes", EmailVerificationHourlySendLimit: 100}
	// settings.Current.LoadKeys()
	handler, err := router.NewHandler(settings.Current, permission_cache.NewPermissionCache(settings.Current), sessionstore.NewSessionStore(settings.Current))

	Ω(err).Should(Succeed())
	verificationService, err := actions.NewEmailVerificationService(models.Dbcon, settings.Current, verificationEmails)
	Ω(err).Should(Succeed())
	ctr.SetEmailVerificationService(verificationService)
	s = httptest.NewServer(handler)
	c = s.Client()
	valids.InitializeValidations()
})

var _ = AfterSuite(func() {
	err := models.Dbcon.Exec(`DROP SCHEMA public CASCADE;CREATE SCHEMA public;`).Error
	Ω(err).To(Succeed())
	cleanupDocker()
})

var _ = BeforeEach(func() {
	// clear db tables before each test
	// err := models.Dbcon.Exec(`DROP SCHEMA public CASCADE;CREATE SCHEMA public;`).Error
	// Ω(err).To(Succeed())
})

const (
	dbName = "bigbucks"
	passwd = "bigbucks"
)

func setupGormWithDocker() (*gorm.DB, func()) {
	pool, err := dockertest.NewPool("")
	chk(err)
	// Start postgres container
	postgresResource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "17",
		Env: []string{
			"POSTGRES_PASSWORD=" + passwd,
			"POSTGRES_DB=" + dbName,
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.NeverRestart()
		config.PortBindings = map[docker.Port][]docker.PortBinding{
			"5432/tcp": {{HostIP: "", HostPort: "7432"}},
		}
	})
	chk(err)

	// Start redis container
	redisResource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "redis",
		Tag:        "8.2",
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.NeverRestart()
		config.PortBindings = map[docker.Port][]docker.PortBinding{
			"6379/tcp": {{HostIP: "", HostPort: "6379"}},
		}
	})
	chk(err)

	// Cleanup function to remove both containers
	fnCleanup := func() {
		err := postgresResource.Close()
		chk(err)
		err = redisResource.Close()
		chk(err)
	}

	conStr := fmt.Sprintf("host=localhost port=%s user=postgres dbname=%s password=%s sslmode=disable",
		"7432",
		dbName,
		passwd,
	)

	var gdb *gorm.DB
	// retry until postgres is ready
	err = pool.Retry(func() error {
		gdb, err = gorm.Open(postgres.Open(conStr), &gorm.Config{})
		if err != nil {
			return err
		}
		db, err := gdb.DB()
		if err != nil {
			return err
		}
		return db.Ping()
	})
	chk(err)

	// retry until redis is ready
	err = pool.Retry(func() error {
		client := redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})
		return client.Ping(context.Background()).Err()
	})
	chk(err)

	// container is ready, return *gorm.Db for testing
	return gdb, fnCleanup
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}
