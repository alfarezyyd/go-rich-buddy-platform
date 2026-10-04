package entity

import "strings"

// WhatsappSession tracks per-phone conversation state machine.
// States: MAIN_MENU | REGISTER_FLOW | ORDER_FLOW | FREE_CHAT_FLOW
type WhatsappSession struct {
	Id                uint64 `gorm:"column:id;primaryKey;autoIncrement"`
	Phone             string `gorm:"column:phone;uniqueIndex"`
	CurrentState      string `gorm:"column:current_state"`
	RetryCount        int    `gorm:"column:retry_count"`
	// LastRadarSymbols menyimpan urutan ticker hasil radar terakhir (comma-separated, e.g. "BBCA,BMRI,TLKM")
	// Digunakan untuk memetakan pilihan angka (1, 2, 3...) ke kode saham yang tepat.
	LastRadarSymbols  string `gorm:"column:last_radar_symbols;default:''"`
	// ActiveSymbol menyimpan ticker saham yang sedang dilihat/di-drilldown oleh user (e.g. "BBCA").
	ActiveSymbol      string `gorm:"column:active_symbol;default:''"`
}

// SetRadarSymbols menyimpan urutan ticker sebagai string comma-separated.
func (s *WhatsappSession) SetRadarSymbols(symbols []string) {
	s.LastRadarSymbols = strings.Join(symbols, ",")
}

// GetRadarSymbols mengembalikan urutan ticker yang tersimpan.
func (s *WhatsappSession) GetRadarSymbols() []string {
	if s.LastRadarSymbols == "" {
		return nil
	}
	return strings.Split(s.LastRadarSymbols, ",")
}

// RadarSymbolByIndex mengembalikan ticker berdasarkan angka pilihan user (1-based).
// Mengembalikan string kosong jika index di luar range.
func (s *WhatsappSession) RadarSymbolByIndex(oneBasedIndex int) string {
	symbols := s.GetRadarSymbols()
	if oneBasedIndex < 1 || oneBasedIndex > len(symbols) {
		return ""
	}
	return symbols[oneBasedIndex-1]
}
