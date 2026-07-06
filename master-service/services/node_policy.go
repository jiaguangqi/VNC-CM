package services

import (
	"strings"

	"github.com/remote-desktop/master-service/models"
)

func HostAllowsUser(host models.Host, username, role string) bool {
	allowedUsers := splitPolicyList(host.AllowedUsers)
	allowedRoles := splitPolicyList(host.AllowedRoles)
	if len(allowedUsers) == 0 && len(allowedRoles) == 0 {
		return true
	}
	return containsPolicyValue(allowedUsers, username) || containsPolicyValue(allowedRoles, role)
}

func splitPolicyList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func containsPolicyValue(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
