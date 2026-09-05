package commands

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/quackdiscord/bot/internal/quack"
)

type caseContextDraft struct {
	Token                   string
	ActorDiscordUserID      string
	GuildID                 string
	ContextChannelDiscordID string
	ContextMessageDiscordID string
	TargetDiscordUserID     string
	Template                quack.TemplateResponse
	Values                  map[string]json.RawMessage
	EvidenceLinks           []string
	Page                    int
	ExpiresAt               time.Time
}

type caseContextDraftStore struct {
	mu     sync.Mutex
	drafts map[string]caseContextDraft
}

var caseContextDrafts = &caseContextDraftStore{drafts: map[string]caseContextDraft{}}

func (s *caseContextDraftStore) put(draft caseContextDraft) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for token, existing := range s.drafts {
		if !existing.ExpiresAt.After(now) {
			delete(s.drafts, token)
		}
	}
	s.drafts[draft.Token] = draft
}

func (s *caseContextDraftStore) get(token string) (caseContextDraft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft, ok := s.drafts[token]
	if !ok || !draft.ExpiresAt.After(time.Now().UTC()) {
		delete(s.drafts, token)
		return caseContextDraft{}, false
	}
	values := make(map[string]json.RawMessage, len(draft.Values))
	for key, value := range draft.Values {
		values[key] = append(json.RawMessage(nil), value...)
	}
	draft.Values = values
	draft.EvidenceLinks = append([]string(nil), draft.EvidenceLinks...)
	return draft, true
}

func (s *caseContextDraftStore) save(draft caseContextDraft) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drafts[draft.Token] = draft
}

func (s *caseContextDraftStore) delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.drafts, token)
}
