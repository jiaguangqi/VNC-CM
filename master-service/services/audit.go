package services

import (
	"encoding/json"
	"log"

	"github.com/google/uuid"

	"github.com/remote-desktop/master-service/database"
	"github.com/remote-desktop/master-service/models"
)

func RecordAudit(actorID, action, targetType, targetID string, metadata map[string]interface{}, ipAddress string) {
	audit := models.AuditLog{
		ActorID:    parseUUIDOrNil(actorID),
		Action:     action,
		TargetType: targetType,
		TargetID:   parseUUIDOrNil(targetID),
		IPAddress:  ipAddress,
	}

	if metadata != nil {
		data, err := json.Marshal(metadata)
		if err == nil {
			audit.Metadata = string(data)
		}
	}

	if err := database.DB.Create(&audit).Error; err != nil {
		log.Printf("写入审计日志失败 action=%s target=%s/%s: %v", action, targetType, targetID, err)
	}
}

func parseUUIDOrNil(value string) uuid.UUID {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil
	}
	return id
}
