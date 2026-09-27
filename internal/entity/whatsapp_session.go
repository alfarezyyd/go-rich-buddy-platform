package entity

// WhatsappSession tracks per-phone conversation state machine.
// States: MAIN_MENU | REGISTER_FLOW | ORDER_FLOW | FREE_CHAT_FLOW
type WhatsappSession struct {
	Id           uint64 `gorm:"column:id;primaryKey;autoIncrement"`
	Phone        string `gorm:"column:phone;uniqueIndex"`
	CurrentState string `gorm:"column:current_state"`
	RetryCount   int    `gorm:"column:retry_count"`
}
