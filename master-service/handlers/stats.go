// handlers/stats.go - Dashboard 统计数据接口
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/remote-desktop/master-service/database"
	"github.com/remote-desktop/master-service/models"
)

// StatsHandler 统计数据处理器
type StatsHandler struct{}

// NewStatsHandler 创建统计处理器
func NewStatsHandler() *StatsHandler {
	return &StatsHandler{}
}

// OverviewResponse 概览统计响应
type OverviewResponse struct {
	TotalDesktops   int64 `json:"total_desktops"`    // 在线桌面数
	TotalHosts      int64 `json:"total_hosts"`       // 在线主机数
	ActiveUsers     int64 `json:"active_users"`      // 最近7天活跃用户
	DesktopWeekPrev int   `json:"desktop_week_prev"` // 上周桌面数（环比）
	HostWeekPrev    int   `json:"host_week_prev"`    // 上周主机数（环比）
	UserWeekPrev    int   `json:"user_week_prev"`    // 上周用户数（环比）
}

// GetOverview 获取概览统计数据
func (h *StatsHandler) GetOverview(c *gin.Context) {
	var resp OverviewResponse
	db := database.DB

	db.Model(&models.Session{}).
		Where("status IN ?", []string{"running", "idle", "starting", "pending"}).
		Where("deleted_at IS NULL").
		Count(&resp.TotalDesktops)

	db.Model(&models.Host{}).
		Where("status = ?", "healthy").
		Where("deleted_at IS NULL").
		Count(&resp.TotalHosts)

	weekAgo := time.Now().AddDate(0, 0, -7)
	db.Model(&models.Session{}).
		Where("created_at >= ?", weekAgo).
		Where("deleted_at IS NULL").
		Distinct("user_id").
		Count(&resp.ActiveUsers)

	prevWeekStart := time.Now().AddDate(0, 0, -14)
	prevWeekEnd := time.Now().AddDate(0, 0, -7)

	var prevDesktops int64
	db.Model(&models.Session{}).
		Where("status IN ?", []string{"running", "idle", "starting", "pending"}).
		Where("created_at >= ? AND created_at < ?", prevWeekStart, prevWeekEnd).
		Where("deleted_at IS NULL").
		Count(&prevDesktops)
	resp.DesktopWeekPrev = int(prevDesktops)

	var prevHosts int64
	db.Model(&models.Host{}).
		Where("status = ?", "healthy").
		Where("deleted_at IS NULL").
		Count(&prevHosts)
	resp.HostWeekPrev = int(prevHosts)

	var prevUsers int64
	db.Model(&models.Session{}).
		Where("created_at >= ? AND created_at < ?", prevWeekStart, prevWeekEnd).
		Where("deleted_at IS NULL").
		Distinct("user_id").
		Count(&prevUsers)
	resp.UserWeekPrev = int(prevUsers)

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data":   resp,
	})
}

// TrendData 单天趋势数据
type TrendData struct {
	Date            string `json:"date"`
	DesktopCount    int64  `json:"desktop_count"`
	HostCount       int64  `json:"host_count"`
	ActiveUserCount int64  `json:"active_user_count"`
}

// TrendResponse 趋势响应
type TrendResponse struct {
	Days int         `json:"days"`
	Data []TrendData `json:"data"`
}

// GetTrend 获取趋势数据
func (h *StatsHandler) GetTrend(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "7")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 || days > 30 {
		days = 7
	}

	var result []TrendData
	now := time.Now()

	for i := days - 1; i >= 0; i-- {
		dayEnd := time.Date(now.Year(), now.Month(), now.Day()-i, 23, 59, 59, 0, now.Location())
		dayDate := dayEnd.Format("2006-01-02")

		var desktopCount int64
		database.DB.Model(&models.Session{}).
			Where("status IN ?", []string{"running", "idle", "starting", "pending"}).
			Where("created_at <= ?", dayEnd).
			Where("deleted_at IS NULL").
			Count(&desktopCount)

		var hostCount int64
		database.DB.Model(&models.Host{}).
			Where("status = ?", "healthy").
			Where("created_at <= ?", dayEnd).
			Where("deleted_at IS NULL").
			Count(&hostCount)

		var userCount int64
		database.DB.Model(&models.Session{}).
			Where("created_at >= ? AND created_at <= ?", dayEnd.AddDate(0, 0, -1), dayEnd).
			Where("deleted_at IS NULL").
			Distinct("user_id").
			Count(&userCount)

		result = append(result, TrendData{
			Date:            dayDate,
			DesktopCount:    desktopCount,
			HostCount:       hostCount,
			ActiveUserCount: userCount,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"data": TrendResponse{
			Days: days,
			Data: result,
		},
	})
}
