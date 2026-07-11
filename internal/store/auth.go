package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/quackdiscord/bot/internal/quack/model"
	r "github.com/redis/go-redis/v9"
)

const (
	oauthStateKeyPrefix  = "auth:state:"
	authSessionKeyPrefix = "auth:session:"
)

// SaveOAuthState stores short-lived OAuth state so the callback can reject unsolicited or replayed login attempts.
func (s *Store) SaveOAuthState(ctx context.Context, state string, payload *model.OAuthState, ttl time.Duration) error {
	if s == nil || s.redis == nil {
		return errors.New("redis not connected")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal oauth state: %w", err)
	}

	if err := s.redis.Set(ctx, oauthStateKeyPrefix+state, body, ttl).Err(); err != nil {
		return fmt.Errorf("save oauth state: %w", err)
	}

	return nil
}

// ConsumeOAuthState atomically reads and deletes OAuth state so one authorization response cannot be replayed.
func (s *Store) ConsumeOAuthState(ctx context.Context, state string) (*model.OAuthState, error) {
	if s == nil || s.redis == nil {
		return nil, errors.New("redis not connected")
	}

	key := oauthStateKeyPrefix + state
	body, err := s.redis.GetDel(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, r.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("read oauth state: %w", err)
	}

	var payload model.OAuthState
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal oauth state: %w", err)
	}

	return &payload, nil
}

// SaveSession persists the complete authentication session with its configured expiry.
func (s *Store) SaveSession(ctx context.Context, session *model.AuthSession, ttl time.Duration) error {
	if s == nil || s.redis == nil {
		return errors.New("redis not connected")
	}

	body, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal auth session: %w", err)
	}

	if err := s.redis.Set(ctx, authSessionKeyPrefix+session.ID, body, ttl).Err(); err != nil {
		return fmt.Errorf("save auth session: %w", err)
	}

	return nil
}

// GetSession loads a session by ID and treats a missing Redis key as an unauthenticated request rather than a storage failure.
func (s *Store) GetSession(ctx context.Context, sessionID string) (*model.AuthSession, error) {
	if s == nil || s.redis == nil {
		return nil, errors.New("redis not connected")
	}

	body, err := s.redis.Get(ctx, authSessionKeyPrefix+sessionID).Bytes()
	if err != nil {
		if errors.Is(err, r.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("read auth session: %w", err)
	}

	var session model.AuthSession
	if err := json.Unmarshal(body, &session); err != nil {
		return nil, fmt.Errorf("unmarshal auth session: %w", err)
	}

	return &session, nil
}

// DeleteSession removes a session during logout or explicit invalidation.
func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	if s == nil || s.redis == nil {
		return errors.New("redis not connected")
	}

	if err := s.redis.Del(ctx, authSessionKeyPrefix+sessionID).Err(); err != nil {
		return fmt.Errorf("delete auth session: %w", err)
	}

	return nil
}

// RefreshSessionTTL extends an active session without rewriting its payload.
func (s *Store) RefreshSessionTTL(ctx context.Context, sessionID string, ttl time.Duration) error {
	if s == nil || s.redis == nil {
		return errors.New("redis not connected")
	}

	if err := s.redis.Expire(ctx, authSessionKeyPrefix+sessionID, ttl).Err(); err != nil {
		return fmt.Errorf("refresh auth session ttl: %w", err)
	}

	return nil
}
