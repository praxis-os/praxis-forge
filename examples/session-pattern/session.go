// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	praxis "github.com/praxis-os/praxis"
	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/llm"

	forge "github.com/praxis-os/praxis-forge"
)

// SessionID is the caller's identifier for a session. Forge never sees it;
// the App below indexes its own Store by SessionID and threads correlation
// tags into telemetry via request Metadata.
type SessionID string

// Session is the caller-owned persistent record for one conversation. It
// carries the full message history, an optional ApprovalSnapshot when the
// invocation is paused on human approval, and bookkeeping timestamps.
//
// Nothing in this type is defined by forge. Adapt it freely to whatever
// storage shape your application uses.
type Session struct {
	ID              SessionID
	Messages        []llm.Message
	PendingApproval *praxiserrors.ApprovalSnapshot
	Updated         time.Time
}

// Store is the caller-defined persistence surface. Production implementations
// back it with a database or a KV store; this example uses memory.
type Store interface {
	Load(ctx context.Context, id SessionID) (*Session, error)
	Save(ctx context.Context, s *Session) error
}

// InMemoryStore is the simplest Store: a mutex-guarded map. Good for demos
// and tests.
type InMemoryStore struct {
	mu   sync.Mutex
	data map[SessionID]*Session
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{data: map[SessionID]*Session{}}
}

func (s *InMemoryStore) Load(_ context.Context, id SessionID) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.data[id]; ok {
		return existing, nil
	}
	return &Session{ID: id}, nil
}

func (s *InMemoryStore) Save(_ context.Context, sess *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess.Updated = time.Now().UTC()
	s.data[sess.ID] = sess
	return nil
}

// App wraps a stateless forge.BuiltAgent with a caller-owned Store to provide
// session continuity. Turn advances a session by one user/assistant exchange;
// Resume continues a paused invocation after human approval.
type App struct {
	Agent *forge.BuiltAgent
	Store Store
	Model string
}

// Turn appends the user input to the session, invokes the agent, and stores
// the resulting assistant message. If the agent pauses for approval, the
// ApprovalSnapshot is persisted on the session and the error is returned so
// the caller can solicit a human decision.
func (a *App) Turn(ctx context.Context, id SessionID, userInput string) (*llm.Message, error) {
	sess, err := a.Store.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	if sess.PendingApproval != nil {
		return nil, fmt.Errorf("session %s: pending approval; call Resume first", id)
	}

	sess.Messages = append(sess.Messages, llm.Message{
		Role:  llm.RoleUser,
		Parts: []llm.MessagePart{llm.TextPart(userInput)},
	})

	res, err := a.Agent.Invoke(ctx, praxis.InvocationRequest{
		Model:        a.Model,
		SystemPrompt: a.Agent.SystemPrompt(),
		Messages:     sess.Messages,
	})

	var approval *praxiserrors.ApprovalRequiredError
	if errors.As(err, &approval) {
		snap := approval.Snapshot
		sess.PendingApproval = &snap
		if saveErr := a.Store.Save(ctx, sess); saveErr != nil {
			return nil, fmt.Errorf("persist pending approval: %w", saveErr)
		}
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	sess.Messages = append(sess.Messages, *res.Response)
	if saveErr := a.Store.Save(ctx, sess); saveErr != nil {
		return nil, fmt.Errorf("persist turn: %w", saveErr)
	}
	return res.Response, nil
}

// Resume re-invokes a paused session with the stored ApprovalSnapshot and an
// approval_decision Metadata entry that the policy hook can use to short-
// circuit its check. The approved=false path denies the pending action and
// clears the pause without advancing the conversation.
func (a *App) Resume(ctx context.Context, id SessionID, approved bool) error {
	sess, err := a.Store.Load(ctx, id)
	if err != nil {
		return err
	}
	if sess.PendingApproval == nil {
		return fmt.Errorf("session %s: no pending approval", id)
	}
	snap := sess.PendingApproval

	if !approved {
		sess.PendingApproval = nil
		return a.Store.Save(ctx, sess)
	}

	res, err := a.Agent.Invoke(ctx, praxis.InvocationRequest{
		Model:        snap.Model,
		SystemPrompt: snap.SystemPrompt,
		Messages:     snap.Messages,
		Metadata:     map[string]string{"approval_decision": "approved"},
	})
	if err != nil {
		// Could pause again with a fresh snapshot; omitted for brevity.
		return err
	}

	sess.Messages = snap.Messages
	sess.Messages = append(sess.Messages, *res.Response)
	sess.PendingApproval = nil
	return a.Store.Save(ctx, sess)
}
