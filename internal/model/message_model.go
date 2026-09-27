package model

type TextMessage struct {
	Event     string             `json:"event"`
	DeviceId  string             `json:"device_id"`
	SessionId string             `json:"session_id"`
	Payload   TextMessagePayload `json:"payload"`
}

type TextMessagePayload struct {
	Id        string `json:"id"`
	ChatId    string `json:"chat_id"`
	From      string `json:"from"`
	FromLid   string `json:"from_lid"`
	FromName  string `json:"from_name"`
	Timestamp string `json:"timestamp"`
	IsFromMe  bool   `json:"is_from_me"`
	Body      string `json:"body"`
}
