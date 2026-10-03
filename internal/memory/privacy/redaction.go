package privacy

import (
	"regexp"
	"strings"
)

// sensitivePatterns lists regular expressions for PII that must be redacted before storing.
// Per PRD §8.2: NIK (16-digit), credit cards, bank accounts, email, phone numbers.
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b\d{16}\b`),                                                   // 16-digit NIK / card numbers
	regexp.MustCompile(`\b\d{10,15}\b`),                                                // bank account / phone numbers (10-15 digits)
	regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),           // email addresses
	regexp.MustCompile(`\b(?:\d[ -]?){13,16}\b`),                                       // spaced/dashed card numbers
}

// RedactSensitiveData replaces known PII patterns with [REDACTED].
func RedactSensitiveData(content string) string {
	for _, pattern := range sensitivePatterns {
		content = pattern.ReplaceAllString(content, "[REDACTED]")
	}
	return content
}

// memoryCommandKeywords defines deterministic keywords for user memory commands per PRD §8.1.
var memoryCommandKeywords = map[string]MemoryCommand{
	"memori saya":      CommandViewMemory,
	"my memory":        CommandViewMemory,
	"hapus semua memori": CommandDeleteAllMemory,
	"delete all memory": CommandDeleteAllMemory,
	"hapus riwayat chat": CommandDeleteChatHistory,
	"delete chat history": CommandDeleteChatHistory,
}

// MemoryCommand represents a recognized user memory control command.
type MemoryCommand int

const (
	CommandNone            MemoryCommand = iota
	CommandViewMemory                    // "memori saya"
	CommandDeleteAllMemory               // "hapus semua memori"
	CommandDeleteChatHistory             // "hapus riwayat chat"
	CommandForgetItem                    // "lupakan <hal>"
)

// ForgetPrefix lists keywords that trigger the forget-item command.
var ForgetPrefix = []string{"lupakan ", "forget "}

// ParseMemoryCommand performs deterministic keyword matching to detect user memory commands.
// Per PRD §8.1, LLM is only used as a fallback interpreter.
func ParseMemoryCommand(body string) (MemoryCommand, string) {
	normalized := strings.TrimSpace(strings.ToLower(body))

	if cmd, ok := memoryCommandKeywords[normalized]; ok {
		return cmd, ""
	}

	for _, prefix := range ForgetPrefix {
		if strings.HasPrefix(normalized, prefix) {
			target := strings.TrimSpace(body[len(prefix):])
			return CommandForgetItem, target
		}
	}

	return CommandNone, ""
}
