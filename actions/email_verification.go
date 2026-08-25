package actions

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"bigbucks/solution/auth/emailservice"
	"bigbucks/solution/auth/models"
	"bigbucks/solution/auth/settings"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrVerificationInvalid   = errors.New("invalid verification code")
	ErrVerificationExpired   = errors.New("verification code expired")
	ErrVerificationLocked    = errors.New("verification attempts exceeded")
	ErrVerificationConsumed  = errors.New("verification code already used")
	ErrVerificationCooldown  = errors.New("verification code was sent recently")
	ErrVerificationEmailSend = errors.New("failed to send verification email")
)

type EmailVerificationService struct {
	db             *gorm.DB
	redis          *redis.Client
	sender         emailservice.EmailSender
	secret         []byte
	ttl            time.Duration
	maxAttempts    uint
	resendCooldown time.Duration
	hourlyLimit    int64
	now            func() time.Time
	random         io.Reader
}

func NewEmailVerificationService(db *gorm.DB, config *settings.Settings, sender emailservice.EmailSender) (*EmailVerificationService, error) {
	ttl, maxAttempts, resendCooldown, hourlyLimit, err := config.EmailVerificationPolicy()
	if err != nil {
		return nil, err
	}

	return &EmailVerificationService{
		db: db,
		redis: redis.NewClient(&redis.Options{
			Addr:     config.RedisAddress,
			Username: config.RedisUsername,
			Password: config.RedisPassword,
		}),
		sender:         sender,
		secret:         []byte(config.EmailVerificationSecret),
		ttl:            ttl,
		maxAttempts:    maxAttempts,
		resendCooldown: resendCooldown,
		hourlyLimit:    hourlyLimit,
		now:            time.Now,
		random:         rand.Reader,
	}, nil
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (service *EmailVerificationService) Register(user *models.User, sourceIP string) error {
	user.Username = NormalizeEmail(user.Username)
	user.Profile.Email = user.Username

	if err := service.enforceSendLimit(context.Background(), user.Username, sourceIP); err != nil {
		return err
	}

	code, err := service.generateCode()
	if err != nil {
		return err
	}
	now := service.now().UTC()

	err = service.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		verification := &models.EmailVerification{
			UserID:       user.ID,
			CodeDigest:   service.digest(user.ID, user.Username, code),
			Email:        user.Username,
			ExpiresAt:    now.Add(service.ttl),
			LastSentAt:   now,
			SendWindowAt: now,
			SendCount:    1,
		}
		return tx.Create(verification).Error
	})
	if err != nil {
		return err
	}

	if err := service.send(user.Username, code); err != nil {
		return fmt.Errorf("%w: %v", ErrVerificationEmailSend, err)
	}
	return nil
}

func (service *EmailVerificationService) Resend(email, sourceIP string) error {
	email = NormalizeEmail(email)
	if err := service.enforceSendLimit(context.Background(), email, sourceIP); err != nil {
		return err
	}

	var user models.User
	if err := service.db.Where("username = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if user.EmailVerified {
		return nil
	}

	code, err := service.generateCode()
	if err != nil {
		return err
	}
	now := service.now().UTC()

	err = service.db.Transaction(func(tx *gorm.DB) error {
		var verification models.EmailVerification
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", user.ID).First(&verification).Error; err != nil {
			return err
		}
		if now.Before(verification.LastSentAt.Add(service.resendCooldown)) {
			return ErrVerificationCooldown
		}

		if now.Sub(verification.SendWindowAt) >= time.Hour {
			verification.SendWindowAt = now
			verification.SendCount = 0
		}
		if int64(verification.SendCount) >= service.hourlyLimit {
			return ErrVerificationLocked
		}

		verification.CodeDigest = service.digest(user.ID, email, code)
		verification.FailedAttempts = 0
		verification.ExpiresAt = now.Add(service.ttl)
		verification.LastSentAt = now
		verification.SendCount++
		verification.ConsumedAt = nil
		return tx.Save(&verification).Error
	})
	if err != nil {
		return err
	}

	if err := service.send(email, code); err != nil {
		return fmt.Errorf("%w: %v", ErrVerificationEmailSend, err)
	}
	return nil
}

func (service *EmailVerificationService) Verify(email, code string) error {
	email = NormalizeEmail(email)
	now := service.now().UTC()
	var verificationError error

	err := service.db.Transaction(func(tx *gorm.DB) error {
		var verification models.EmailVerification
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("email = ?", email).First(&verification).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrVerificationInvalid
			}
			return err
		}
		if verification.ConsumedAt != nil {
			return ErrVerificationConsumed
		}
		if !now.Before(verification.ExpiresAt) {
			return ErrVerificationExpired
		}
		if verification.FailedAttempts >= service.maxAttempts {
			return ErrVerificationLocked
		}
		if !hmac.Equal(verification.CodeDigest, service.digest(verification.UserID, email, code)) {
			verification.FailedAttempts++
			if err := tx.Save(&verification).Error; err != nil {
				return err
			}
			if verification.FailedAttempts >= service.maxAttempts {
				verificationError = ErrVerificationLocked
				return nil
			}
			verificationError = ErrVerificationInvalid
			return nil
		}

		verification.ConsumedAt = &now
		if err := tx.Save(&verification).Error; err != nil {
			return err
		}
		result := tx.Model(&models.User{}).Where("id = ? AND email_verified = ?", verification.UserID, false).Update("email_verified", true)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrVerificationConsumed
		}
		return nil
	})
	if err != nil {
		return err
	}
	return verificationError
}

func (service *EmailVerificationService) generateCode() (string, error) {
	var digits [6]byte
	for index := range digits {
		var sample [1]byte
		for {
			if _, err := io.ReadFull(service.random, sample[:]); err != nil {
				return "", err
			}
			if sample[0] < 250 {
				digits[index] = '0' + sample[0]%10
				break
			}
		}
	}
	return string(digits[:]), nil
}

func (service *EmailVerificationService) digest(userID, email, code string) []byte {
	mac := hmac.New(sha256.New, service.secret)
	_, _ = fmt.Fprintf(mac, "email-verification\x00%s\x00%s\x00%s", userID, email, code)
	return mac.Sum(nil)
}

func (service *EmailVerificationService) enforceSendLimit(ctx context.Context, email, sourceIP string) error {
	if service.redis == nil || service.redis.Options().Addr == "" {
		return nil
	}

	for _, identity := range []string{"email:" + NormalizeEmail(email), "ip:" + sourceIP} {
		hash := sha256.Sum256([]byte(identity))
		key := "email-verification:send:" + hex.EncodeToString(hash[:])
		count, err := service.redis.Eval(ctx, `
local count = redis.call('INCR', KEYS[1])
if count == 1 then redis.call('EXPIRE', KEYS[1], ARGV[1]) end
return count
`, []string{key}, int(time.Hour.Seconds())).Int64()
		if err != nil {
			return err
		}
		if count > service.hourlyLimit {
			return ErrVerificationLocked
		}
	}
	return nil
}

func (service *EmailVerificationService) send(email, code string) error {
	return service.sender.SendEmailWithSubject(email, "Verify your BigBucks email", "./templates/email_verification.html", map[string]any{
		"Code":         code,
		"ExpiresInMin": int(service.ttl.Minutes()),
	})
}
