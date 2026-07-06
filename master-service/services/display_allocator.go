package services

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/remote-desktop/master-service/database"
	"github.com/remote-desktop/master-service/models"
)

func AvailableDisplaysForHost(hostID uuid.UUID, maxSessions int) ([]int, error) {
	if maxSessions <= 0 {
		maxSessions = 10
	}

	var sessions []models.Session
	if err := database.DB.
		Where("host_id = ? AND status NOT IN ?", hostID, []string{models.SessionStatusTerminated, models.SessionStatusError}).
		Find(&sessions).Error; err != nil {
		return nil, err
	}

	occupied := make(map[int]struct{}, len(sessions))
	for _, session := range sessions {
		display, ok := sessionDisplay(session)
		if ok && display > 0 {
			occupied[display] = struct{}{}
		}
	}

	available := make([]int, 0, maxSessions)
	for display := 1; display <= maxSessions; display++ {
		if _, exists := occupied[display]; !exists {
			available = append(available, display)
		}
	}

	if len(available) == 0 {
		return nil, fmt.Errorf("宿主机可用 display 已用尽")
	}
	return available, nil
}

func sessionDisplay(session models.Session) (int, bool) {
	if session.ConnectionInfo == "" {
		return 0, false
	}

	var connInfo map[string]interface{}
	if err := json.Unmarshal([]byte(session.ConnectionInfo), &connInfo); err != nil {
		return 0, false
	}

	value, ok := connInfo["display"]
	if !ok {
		return 0, false
	}

	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case json.Number:
		i, err := v.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}
