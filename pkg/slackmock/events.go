package slackmock

import "encoding/json"

// DMMessage builds a Socket Mode events_api payload for a direct message.
func DMMessage(userID, channelID, text string) json.RawMessage {
	payload := map[string]any{
		"token":      "verification-token",
		"team_id":    "T111",
		"api_app_id": "A111",
		"event": map[string]any{
			"type":         "message",
			"text":         text,
			"user":         userID,
			"channel":      channelID,
			"channel_type": "im",
			"ts":           "1610241741.000200",
			"event_ts":     "1610241741.000200",
		},
		"type":       "event_callback",
		"event_id":   "Ev111",
		"event_time": 1610241741,
	}
	b, _ := json.Marshal(payload)
	return b
}
