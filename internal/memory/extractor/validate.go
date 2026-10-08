package extractor

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
)

// ErrKeyNotAllowed is returned when an extracted operation references a key not on the whitelist.
var ErrKeyNotAllowed = errors.New("memory key not in whitelist")

// ErrValueRejected is returned when a value fails enum validation, length check,
// or contains directive patterns that could alter agent behavior.
var ErrValueRejected = errors.New("memory value rejected by validation")

// Operation represents a single memory operation returned by the LLM extractor.
type Operation struct {
	Op                string  `json:"op"`   // "add" | "update" | "delete"
	Key               string  `json:"key"`
	Value             string  `json:"value"`
	Source            string  `json:"source"` // "stated" | "inferred"
	Confidence        float64 `json:"confidence"`
	EvidenceMessageID string  `json:"evidence_message_id"`
}

// ExtractionResult is the structured output from the LLM memory extractor.
type ExtractionResult struct {
	Operations []Operation `json:"operations"`
}

// allowedKeys is the whitelist of permitted memory keys mapped to their value validators.
// Per PRD §4.1 — only these keys may be stored; any others are silently discarded.
var allowedKeys = map[string]func(string) bool{
	"profile.experience_level": oneOf("pemula", "menengah", "mahir"),
	"pref.explanation_depth":   oneOf("ringkas", "standar", "detail"),
	"pref.tone":                oneOf("santai", "formal"),
	"pref.emoji":               oneOf("on", "off"),
	"pref.language":            oneOf("id", "en"),
	"interest.topics":          validTopicList,
	"goal.learning":            maxLen(100),
}

// directivePatterns detects values that attempt to inject behavioral instructions into the agent.
// Per PRD §6.3 rule 4.
var directivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(selalu|always)\s+(sarankan|suggest|rekomendasikan|recommend)`),
	regexp.MustCompile(`(?i)(abaikan|ignore|bypass|jangan ikuti)\s+(aturan|rule|instruksi|instruction)`),
	regexp.MustCompile(`(?i)(act as|berperan sebagai|pretend)`),
	regexp.MustCompile(`(?i)(system prompt|override|jailbreak)`),
}

// Validate checks a single Operation against the whitelist, enum constraints, and injection rules.
func Validate(op Operation) error {
	checker, ok := allowedKeys[op.Key]
	if !ok {
		return ErrKeyNotAllowed
	}

	sanitizedValue := sanitizeValue(op.Value)
	if sanitizedValue == "" {
		return ErrValueRejected
	}

	if !checker(sanitizedValue) {
		return ErrValueRejected
	}

	if containsDirective(sanitizedValue) {
		return ErrValueRejected
	}

	// Reject values that hold personal financial or identity data.
	if containsPersonalData(sanitizedValue) {
		return ErrValueRejected
	}

	return nil
}

// SanitizedValue returns the clean value after stripping control characters.
func SanitizedValue(value string) string {
	return sanitizeValue(value)
}

// sanitizeValue strips control characters and normalizes whitespace from a memory value.
func sanitizeValue(value string) string {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\t' {
			return -1
		}
		return r
	}, value)
	return strings.TrimSpace(cleaned)
}

// containsDirective returns true if the value appears to be a behavioral instruction.
func containsDirective(value string) bool {
	for _, pattern := range directivePatterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

// containsPersonalData returns true if the value contains personal financial or identity data.
func containsPersonalData(value string) bool {
	personalPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\b\d{16}\b`),
		regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
		regexp.MustCompile(`\b\d{10,15}\b`),
	}
	for _, p := range personalPatterns {
		if p.MatchString(value) {
			return true
		}
	}
	return false
}

// oneOf returns a validator that checks the value is one of the provided enum values.
func oneOf(values ...string) func(string) bool {
	allowed := make(map[string]struct{}, len(values))
	for _, v := range values {
		allowed[v] = struct{}{}
	}
	return func(s string) bool {
		_, ok := allowed[s]
		return ok
	}
}

// maxLen returns a validator that checks the value does not exceed maxCharacters.
func maxLen(maxCharacters int) func(string) bool {
	return func(s string) bool {
		return len([]rune(s)) <= maxCharacters
	}
}

// validTopicList validates a comma-separated list of topic tags.
// Each tag must be non-empty and may only contain letters, digits, spaces, and hyphens.
func validTopicList(value string) bool {
	tags := strings.Split(value, ",")
	if len(tags) == 0 || len(tags) > 20 {
		return false
	}
	tagPattern := regexp.MustCompile(`^[a-zA-Z0-9\s\-]{1,50}$`)
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" || !tagPattern.MatchString(trimmed) {
			return false
		}
	}
	return true
}

// IsStated returns true when the operation source is "stated" (user explicitly said it).
func IsStated(op Operation) bool {
	return strings.EqualFold(op.Source, "stated")
}
