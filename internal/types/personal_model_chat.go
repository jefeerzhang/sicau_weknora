package types

import "context"

// PersonalModelIDPrefix marks a chat model ID that resolves via 学生个人模型
// credentials rather than the workspace models table (ADR-0001).
const PersonalModelIDPrefix = "pm:"

// FormatPersonalModelRef builds the chat-override ID for a personal model row.
func FormatPersonalModelRef(id string) string {
	if id == "" {
		return ""
	}
	if len(id) >= len(PersonalModelIDPrefix) && id[:len(PersonalModelIDPrefix)] == PersonalModelIDPrefix {
		return id
	}
	return PersonalModelIDPrefix + id
}

// ParsePersonalModelRef returns the row id when s is a personal model ref.
func ParsePersonalModelRef(s string) (id string, ok bool) {
	if len(s) <= len(PersonalModelIDPrefix) || s[:len(PersonalModelIDPrefix)] != PersonalModelIDPrefix {
		return "", false
	}
	id = s[len(PersonalModelIDPrefix):]
	if id == "" {
		return "", false
	}
	return id, true
}

type personalChatConfigKey struct{}

// PersonalChatResolved carries decrypted credentials for one turn.
type PersonalChatResolved struct {
	ID        string
	ModelName string
	BaseURL   string
	Provider  string
	APIKey    string
}

// WithPersonalChatResolved attaches personal chat credentials to ctx for
// ModelService.GetChatModel. Missing attachment must fail the turn (no school-bill fallback).
func WithPersonalChatResolved(ctx context.Context, r *PersonalChatResolved) context.Context {
	if r == nil {
		return ctx
	}
	return context.WithValue(ctx, personalChatConfigKey{}, r)
}

// PersonalChatResolvedFromContext returns the turn-scoped personal chat config.
func PersonalChatResolvedFromContext(ctx context.Context) *PersonalChatResolved {
	if ctx == nil {
		return nil
	}
	r, _ := ctx.Value(personalChatConfigKey{}).(*PersonalChatResolved)
	return r
}
