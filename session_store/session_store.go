package sessionstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bigbucks/solution/auth/settings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	MaxSessionsPerUser = 5 // Maximum number of active sessions per user
	SessionKeyPrefix   = "session:"
	UserSessionsPrefix = "user-sessions:"
)

// SessionData represents the data stored in Redis for each session
type SessionData struct {
	UserID    string    `json:"userId"`
	Username  string    `json:"username"`
	UserAgent string    `json:"userAgent"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"createdAt"`
	LastSeen  time.Time `json:"lastSeen"`
}

// SessionStore manages user sessions using Redis
type SessionStore struct {
	client *redis.Client
	ctx    context.Context
}

// NewSessionStore creates a new session store with the provided settings
func NewSessionStore(settings *settings.Settings) *SessionStore {
	client := redis.NewClient(&redis.Options{
		Addr:     settings.RedisAddress,
		Username: settings.RedisUsername,
		Password: settings.RedisPassword,
		DB:       0, // use default DB
	})

	return &SessionStore{
		client: client,
		ctx:    context.Background(),
	}
}

// CreateSession creates a new session for a user and stores it in Redis
func (s *SessionStore) CreateSession(userID, username, userAgent, ip string, expiresIn time.Duration) (string, error) {
	// Generate a new session ID
	sessionID := uuid.New().String()

	// Create session data
	sessionData := SessionData{
		UserID:    userID,
		Username:  username,
		UserAgent: userAgent,
		IP:        ip,
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
	}

	// Serialize session data
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return "", err
	}

	// Get the user's current sessions
	userSessionsKey := fmt.Sprintf("%s%s", UserSessionsPrefix, userID)

	// Use a Redis transaction to manage the session count
	pipe := s.client.TxPipeline()

	// Add session to the sorted set with creation time as score
	pipe.ZAdd(s.ctx, userSessionsKey, redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: sessionID,
	})

	// Check if user has too many sessions
	countCmd := pipe.ZCard(s.ctx, userSessionsKey)

	// Execute the pipeline
	_, err = pipe.Exec(s.ctx)
	if err != nil {
		return "", err
	}

	// If user has too many sessions, remove the oldest one
	count := countCmd.Val()
	if count > MaxSessionsPerUser {
		// Get the oldest session
		oldestSessions, err := s.client.ZRangeWithScores(s.ctx, userSessionsKey, 0, 0).Result()
		if err != nil {
			return "", err
		}

		if len(oldestSessions) > 0 {
			oldestSessionID := oldestSessions[0].Member.(string)

			// Remove the oldest session
			pipe := s.client.TxPipeline()
			pipe.ZRem(s.ctx, userSessionsKey, oldestSessionID)
			pipe.Del(s.ctx, fmt.Sprintf("%s%s", SessionKeyPrefix, oldestSessionID))
			_, err = pipe.Exec(s.ctx)
			if err != nil {
				return "", err
			}
		}
	}

	// Store the session data with expiration
	sessionKey := fmt.Sprintf("%s%s", SessionKeyPrefix, sessionID)
	err = s.client.Set(s.ctx, sessionKey, sessionJSON, expiresIn).Err()
	if err != nil {
		return "", err
	}

	// Set expiration on the user sessions set if it's new
	if count == 1 {
		// Set a longer expiration on the user sessions set (e.g., 30 days)
		s.client.Expire(s.ctx, userSessionsKey, 30*24*time.Hour)
	}

	return sessionID, nil
}

// GetSession retrieves session data from Redis
func (s *SessionStore) GetSession(sessionID string) (*SessionData, error) {
	sessionKey := fmt.Sprintf("%s%s", SessionKeyPrefix, sessionID)

	// Get session data from Redis
	sessionJSON, err := s.client.Get(s.ctx, sessionKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, errors.New("session not found")
		}
		return nil, err
	}

	// Deserialize session data
	var sessionData SessionData
	err = json.Unmarshal([]byte(sessionJSON), &sessionData)
	if err != nil {
		return nil, err
	}

	return &sessionData, nil
}

// ValidateSession checks if a session is valid and updates last seen time
func (s *SessionStore) ValidateSession(sessionID string) (bool, *SessionData, error) {
	sessionData, err := s.GetSession(sessionID)
	if err != nil {
		return false, nil, err
	}

	// Update last seen time
	sessionData.LastSeen = time.Now()
	sessionJSON, err := json.Marshal(sessionData)
	if err != nil {
		return false, nil, err
	}

	// Get the TTL of the existing session
	sessionKey := fmt.Sprintf("%s%s", SessionKeyPrefix, sessionID)
	ttl, err := s.client.TTL(s.ctx, sessionKey).Result()
	if err != nil {
		return false, nil, err
	}

	// Update the session with the new last seen time but keep the same TTL
	err = s.client.Set(s.ctx, sessionKey, sessionJSON, ttl).Err()
	if err != nil {
		return false, nil, err
	}

	return true, sessionData, nil
}

// RevokeSession invalidates a specific session
func (s *SessionStore) RevokeSession(sessionID string) error {
	// Get the session first to find the user ID
	sessionData, err := s.GetSession(sessionID)
	if err != nil {
		return err
	}

	// Use a transaction to remove both the session and its reference in the user's sessions
	pipe := s.client.TxPipeline()

	// Remove session from Redis
	sessionKey := fmt.Sprintf("%s%s", SessionKeyPrefix, sessionID)
	pipe.Del(s.ctx, sessionKey)

	// Remove session from user's sessions set
	userSessionsKey := fmt.Sprintf("%s%s", UserSessionsPrefix, sessionData.UserID)
	pipe.ZRem(s.ctx, userSessionsKey, sessionID)

	_, err = pipe.Exec(s.ctx)
	return err
}

// RevokeAllUserSessions invalidates all sessions for a user except the current one
func (s *SessionStore) RevokeAllUserSessions(userID string, exceptSessionID string) error {
	userSessionsKey := fmt.Sprintf("%s%s", UserSessionsPrefix, userID)

	// Get all session IDs for the user
	sessionIDs, err := s.client.ZRange(s.ctx, userSessionsKey, 0, -1).Result()
	if err != nil {
		return err
	}

	// Use a pipeline for efficiency
	pipe := s.client.Pipeline()

	// Delete each session except the current one
	for _, sessionID := range sessionIDs {
		if sessionID != exceptSessionID {
			sessionKey := fmt.Sprintf("%s%s", SessionKeyPrefix, sessionID)
			pipe.Del(s.ctx, sessionKey)
			pipe.ZRem(s.ctx, userSessionsKey, sessionID)
		}
	}

	_, err = pipe.Exec(s.ctx)
	return err
}

// ListUserSessions returns all active sessions for a user
func (s *SessionStore) ListUserSessions(userID string) ([]map[string]interface{}, error) {
	userSessionsKey := fmt.Sprintf("%s%s", UserSessionsPrefix, userID)

	// Get all session IDs for the user
	sessionIDs, err := s.client.ZRange(s.ctx, userSessionsKey, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	sessions := make([]map[string]interface{}, 0, len(sessionIDs))

	// Get data for each session
	for _, sessionID := range sessionIDs {
		sessionKey := fmt.Sprintf("%s%s", SessionKeyPrefix, sessionID)

		// Get session data
		sessionJSON, err := s.client.Get(s.ctx, sessionKey).Result()
		if err != nil {
			if err == redis.Nil {
				// Session expired but still in the set, remove it
				s.client.ZRem(s.ctx, userSessionsKey, sessionID)
				continue
			}
			return nil, err
		}

		// Get TTL for the session
		ttl, err := s.client.TTL(s.ctx, sessionKey).Result()
		if err != nil {
			return nil, err
		}

		// Parse session data
		var sessionData SessionData
		err = json.Unmarshal([]byte(sessionJSON), &sessionData)
		if err != nil {
			return nil, err
		}

		// Create session info with ID and expiration
		sessionInfo := map[string]interface{}{
			"id":         sessionID,
			"userAgent":  sessionData.UserAgent,
			"ip":         sessionData.IP,
			"createdAt":  sessionData.CreatedAt,
			"lastSeen":   sessionData.LastSeen,
			"expiresIn":  ttl.Seconds(),
			"isExpiring": ttl.Seconds() < 3600, // Flag if expiring in less than an hour
		}

		sessions = append(sessions, sessionInfo)
	}

	return sessions, nil
}

// Close closes the Redis client connection
func (s *SessionStore) Close() error {
	return s.client.Close()
}

// Ping checks if the Redis connection is working
func (s *SessionStore) Ping() error {
	return s.client.Ping(s.ctx).Err()
}
