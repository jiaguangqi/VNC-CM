// services/scheduler.go - 桌面调度器骨架

package services

import (
	"fmt"
	"sync/atomic"
)

// Scheduler 调度器接口
type Scheduler interface {
	SelectHost(req *DesktopRequest, hosts []*HostInfo) (*HostInfo, error)
}

// DesktopRequest 用户桌面申请参数
type DesktopRequest struct {
	Protocol          string
	Resolution        string
	ColorDepth        int
	RequireGPU        bool
	RequestedGPUCount int
	UserID            string
}

// HostInfo 宿主机可用信息（用于调度决策）
type HostInfo struct {
	HostID               string
	Hostname             string
	IPAddress            string
	OSType               string
	MaxSessions          int
	CurrentSessions      int
	CPUUsagePercent      float64
	AvailableRAMMB       int64
	GPUCount             int
	AvailableGPUMemoryMB int64
	Region               string
	AZ                   string
	Protocols            []string // 该宿主机支持的协议列表
}

// SchedulerImpl 调度器实现
type SchedulerImpl struct {
	strategy Strategy
}

// Strategy 调度策略接口
type Strategy interface {
	Filter(req *DesktopRequest, hosts []*HostInfo) []*HostInfo
	Score(req *DesktopRequest, host *HostInfo) float64
}

// NewScheduler 创建默认调度器（LeastSessions 策略）
func NewScheduler() Scheduler {
	return &SchedulerImpl{
		strategy: &LeastSessionsStrategy{},
	}
}

// SelectHost 根据策略过滤并选择最优宿主机
func (s *SchedulerImpl) SelectHost(req *DesktopRequest, hosts []*HostInfo) (*HostInfo, error) {
	candidates := s.strategy.Filter(req, hosts)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("没有可用的宿主机满足筛选条件")
	}

	var best *HostInfo
	bestScore := -1.0
	for _, h := range candidates {
		score := s.strategy.Score(req, h)
		if score > bestScore {
			bestScore = score
			best = h
		}
	}
	return best, nil
}

// LeastSessionsStrategy 最少会话数优先策略
type LeastSessionsStrategy struct{}

// Filter 排除不可用的宿主机
func (l *LeastSessionsStrategy) Filter(req *DesktopRequest, hosts []*HostInfo) []*HostInfo {
	filtered := make([]*HostInfo, 0, len(hosts))
	for _, h := range hosts {
		// 检查协议支持
		if !contains(h.Protocols, req.Protocol) {
			continue
		}
		// 检查会话上限
		if h.CurrentSessions >= h.MaxSessions {
			continue
		}
		// 检查 GPU 需求
		if req.RequireGPU && h.GPUCount < req.RequestedGPUCount {
			continue
		}
		filtered = append(filtered, h)
	}
	return filtered
}

// Score 分数越高越优先，当前会话数越少分数越高
func (l *LeastSessionsStrategy) Score(req *DesktopRequest, host *HostInfo) float64 {
	// 基础分：剩余可用槽位比例
	available := host.MaxSessions - host.CurrentSessions
	if available <= 0 {
		return 0
	}
	score := float64(available) / float64(host.MaxSessions)

	// 负载修正：CPU 使用率越低，略微加分
	cpuFactor := 1.0 - (host.CPUUsagePercent / 100.0)
	score *= (0.8 + 0.2*cpuFactor)

	return score
}

// RoundRobinStrategy 轮询策略（可扩展）
type RoundRobinStrategy struct {
	counter uint64
}

func (r *RoundRobinStrategy) Filter(req *DesktopRequest, hosts []*HostInfo) []*HostInfo {
	// 复用 LeastSessions 的筛选逻辑
	return (&LeastSessionsStrategy{}).Filter(req, hosts)
}

func (r *RoundRobinStrategy) Score(req *DesktopRequest, host *HostInfo) float64 {
	// 轮询不计算复杂分数，仅返回固定序列分
	idx := atomic.AddUint64(&r.counter, 1)
	return float64(idx)
}

// utils
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
