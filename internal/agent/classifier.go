package agent

import (
	"context"
	"go-rich-buddy-platform/client"
	"go-rich-buddy-platform/internal/model"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type AgentMode string

const (
	ModeRegular AgentMode = "regular"
	ModeAnalyst AgentMode = "analyst"
)

type Classifier interface {
	Classify(ctx context.Context, userMessage string) AgentMode
}

type HybridClassifier struct {
	agentClient   client.AgentClient
	modelName     string
	tickerPattern *regexp.Regexp
}

func NewClassifier(agentClient client.AgentClient, viperConfig *viper.Viper) Classifier {
	return NewHybridClassifier(agentClient, viperConfig.GetString("AGENT_REGULAR"))
}

func NewHybridClassifier(agentClient client.AgentClient, modelName string) *HybridClassifier {
	return &HybridClassifier{
		agentClient:   agentClient,
		modelName:     modelName,
		tickerPattern: regexp.MustCompile(`\b[A-Za-z]{4}\b`),
	}
}

var knownTickers = map[string]struct{}{
	"bbca": {}, "bbri": {}, "bmri": {}, "bbni": {}, "tlkm": {}, "asii": {},
	"goto": {}, "unvr": {}, "icbp": {}, "indf": {}, "ptba": {}, "adro": {},
	"bren": {}, "ammn": {}, "tpia": {}, "cpin": {}, "jpfa": {}, "pgas": {},
	"klbf": {}, "mika": {}, "smgr": {}, "inkp": {}, "antm": {}, "medc": {},
	"buka": {}, "brpt": {}, "aces": {}, "myor": {}, "heso": {}, "arto": {},
}

var analystKeywords = []string{

	"harga", "price", "closing", "opening", "laporan", "report", "dividend", "dividen",
	"yield", "top gainer", "top loser", "gainers", "losers", "market cap",
	"kapitalisasi", "valuasi", "valuation", "p/e", "per", "pbv", "p/bv",
	"roe", "roa", "eps", "der", "laba", "profit", "rugi", "revenue", "pendapatan",
	"kinerja", "keuangan", "finansial", "volume", "transaksi", "net income", "arus kas",
	"cash flow", "saham", "emiten", "ticker", "idx", "ihsg", "bursa",

	"bandingkan", "perbandingan", "mana yang lebih", "lebih murah", "lebih mahal",
	"lebih tinggi", "lebih bagus", "simulasi", "target price", "prospek angka",
}

var regularGreetings = []string{
	"halo", "hai", "hi", "hello", "hey", "selamat pagi", "selamat siang",
	"selamat sore", "selamat malam", "assalamualaikum", "pagi", "siang", "sore", "malam",
}

var regularChitChat = []string{
	"siapa kamu", "siapa namamu", "kamu siapa", "bisa apa", "bisa bantu apa",
	"terima kasih", "makasih", "thank you", "thanks", "semangat", "motivasi",
}

var regularEducationStarters = []string{
	"apa itu", "apa maksud", "apa definisi", "gimana cara", "bagaimana cara",
	"jelaskan konsep", "pengertian",
}

func (hybridClassifier *HybridClassifier) matchRule(userMessage string) (AgentMode, bool) {
	lowerMsg := strings.ToLower(strings.TrimSpace(userMessage))

	hasAnalystSignal := false
	for _, keyword := range analystKeywords {
		if strings.Contains(lowerMsg, keyword) {
			hasAnalystSignal = true
			break
		}
	}

	if !hasAnalystSignal {
		words := strings.Fields(lowerMsg)
		for _, word := range words {
			cleanWord := strings.Trim(word, ",.?!:;\"'")
			if _, exists := knownTickers[cleanWord]; exists {
				hasAnalystSignal = true
				break
			}
		}
	}

	if !hasAnalystSignal {
		matches := hybridClassifier.tickerPattern.FindAllString(userMessage, -1)
		for _, match := range matches {
			if match == strings.ToUpper(match) {
				hasAnalystSignal = true
				break
			}
		}
	}

	hasRegularSignal := false
	for _, greeting := range regularGreetings {
		if strings.Contains(lowerMsg, greeting) {
			hasRegularSignal = true
			break
		}
	}

	if !hasRegularSignal {
		for _, chitchat := range regularChitChat {
			if strings.Contains(lowerMsg, chitchat) {
				hasRegularSignal = true
				break
			}
		}
	}

	if !hasRegularSignal {
		for _, edu := range regularEducationStarters {
			if strings.Contains(lowerMsg, edu) {
				hasRegularSignal = true
				break
			}
		}
	}

	if hasRegularSignal && !hasAnalystSignal {
		return ModeRegular, true
	}

	if hasAnalystSignal && !hasRegularSignal {
		return ModeAnalyst, true
	}

	return "", false
}

func (hybridClassifier *HybridClassifier) Classify(ctx context.Context, userMessage string) AgentMode {

	if mode, ok := hybridClassifier.matchRule(userMessage); ok {
		logrus.WithFields(logrus.Fields{
			"source": "rule_based",
			"mode":   mode,
		}).Debug("Classified mode via rule-based")
		return mode
	}

	logrus.WithField("user_message", userMessage).Debug("Rule-based ambiguous, using LLM classifier fallback")

	if hybridClassifier.agentClient == nil {
		logrus.Warn("LLM client not configured in classifier, defaulting to analyst per BR-2.5")
		return ModeAnalyst
	}

	classifyRequest := &model.ChatCompletionRequest{
		Model: hybridClassifier.modelName,
		Messages: []model.ChatMessage{
			{
				Role: "system",
				Content: `You are a strict routing classifier for a financial assistant.
Determine whether the user message belongs to "regular" or "analyst" mode.
- "regular": Casual chit-chat, greetings, general concepts without asking for specific stock numbers or market data.
- "analyst": Asking for stock tickers, prices, financial metrics, comparisons, market data, or company reports.
Respond with EXACTLY ONE word: "regular" or "analyst".`,
			},
			{
				Role:    "user",
				Content: userMessage,
			},
		},
	}

	response, err := hybridClassifier.agentClient.CreateChatCompletion(ctx, classifyRequest)
	if err != nil {
		logrus.WithError(err).Warn("LLM classifier failed, defaulting to analyst per BR-2.5")
		return ModeAnalyst
	}

	if len(response.Choices) == 0 {
		return ModeAnalyst
	}

	reply := strings.ToLower(strings.TrimSpace(response.Choices[0].Message.Content))
	if strings.Contains(reply, "regular") {
		logrus.WithField("mode", ModeRegular).Debug("Classified mode via LLM fallback")
		return ModeRegular
	}

	logrus.WithField("mode", ModeAnalyst).Debug("Classified mode via LLM fallback")
	return ModeAnalyst
}
