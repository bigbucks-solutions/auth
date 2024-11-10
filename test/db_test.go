package auth_test

import (
	"bigbucks/solution/auth/models"
	router "bigbucks/solution/auth/rest-api"
	"bigbucks/solution/auth/settings"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	valids "bigbucks/solution/auth/validations"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestDocker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Auth Suite")
}

var (
	DB            *gorm.DB
	cleanupDocker func()
	s             *httptest.Server
	c             *http.Client
)

var _ = BeforeSuite(func() {
	// setup *gorm.Db with docker
	models.Dbcon, cleanupDocker = setupGormWithDocker()

	err := models.Dbcon.SetupJoinTable(&models.Organization{}, "Users", &models.UserOrgRole{})
	err = models.Dbcon.SetupJoinTable(&models.User{}, "Roles", &models.UserOrgRole{})
	models.Migrate()
	GinkgoWriter.Println("Migration complete", err)

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ecder, err := x509.MarshalECPrivateKey(priv)
	settings.SingingKey = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: ecder})
	x509EncodedPub, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	settings.VerifyingKey = pem.EncodeToMemory(&pem.Block{Type: "EC PUBLIC KEY", Bytes: x509EncodedPub})
	sampleData := &models.User{Username: "john@x.com", Password: "john123", Profile: models.Profile{
		FirstName: "John", LastName: "Doe", Email: "john@x.com"},
	}
	err = models.Dbcon.Create(sampleData).Error
	Ω(err).To(Succeed())
	settings.Current = &settings.Settings{Alg: "ES256", PrivateKey: "ec_private.pem", PublicKey: "ec_public.pem"}
	// settings.Current.LoadKeys()
	handler, err := router.NewHandler(settings.Current)
	Ω(err).Should(Succeed())
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

	runDockerOpt := &dockertest.RunOptions{
		Repository: "postgres", // image
		Tag:        "14",       // version
		Env:        []string{"POSTGRES_PASSWORD=" + passwd, "POSTGRES_DB=" + dbName},
	}

	fnConfig := func(config *docker.HostConfig) {
		config.AutoRemove = true                     // set AutoRemove to true so that stopped container goes away by itself
		config.RestartPolicy = docker.NeverRestart() // don't restart container
		config.PortBindings = map[docker.Port][]docker.PortBinding{
			"5432/tcp": {{HostIP: "", HostPort: "6432"}},
		}
	}

	resource, err := pool.RunWithOptions(runDockerOpt, fnConfig)
	chk(err)
	// call clean up function to release resource
	fnCleanup := func() {
		err := resource.Close()
		chk(err)
	}

	conStr := fmt.Sprintf("host=localhost port=%s user=postgres dbname=%s password=%s sslmode=disable",
		"6432", // get port of localhost
		dbName,
		passwd,
	)

	var gdb *gorm.DB
	// retry until db server is ready
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

	// container is ready, return *gorm.Db for testing
	return gdb, fnCleanup
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}
