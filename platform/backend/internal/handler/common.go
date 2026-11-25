package handler

import (
	"encoding/json"
	"strconv"
	"time"

	"polaris-platform/internal/config"
	"polaris-platform/internal/model"

	"gorm.io/gorm"
)

type Handler struct {
	DB        *gorm.DB
	Cfg       *config.Config
	JWTSecret string
}

func New(db *gorm.DB, cfg *config.Config, jwtSecret string) *Handler {
	return &Handler{
		DB:        db,
		Cfg:       cfg,
		JWTSecret: jwtSecret,
	}
}

func (h *Handler) offlineThreshold() time.Duration {
	var s model.Setting
	if err := h.DB.Where("key = ?", "heartbeat_timeout_sec").First(&s).Error; err == nil {
		if sec, err := strconv.Atoi(s.Value); err == nil && sec >= 15 {
			return time.Duration(sec) * time.Second
		}
	}
	return time.Duration(h.Cfg.OfflineSeconds) * time.Second
}

func (h *Handler) isDeviceOnline(lastSeen time.Time) bool {
	if lastSeen.IsZero() {
		return false
	}
	return time.Since(lastSeen) <= h.offlineThreshold()
}

func (h *Handler) getSetting(key, fallback string) string {
	var s model.Setting
	if err := h.DB.Where("key = ?", key).First(&s).Error; err == nil && s.Value != "" {
		return s.Value
	}
	return fallback
}

func (h *Handler) bumpGlobalStrategyTimestamp() int64 {
	ts := time.Now().Unix()
	h.DB.Save(&model.Setting{Key: "strategy_updated_at", Value: strconv.FormatInt(ts, 10)})
	return ts
}

func (h *Handler) resolveDeviceStrategy(dev *model.Device) (map[string]string, int64) {
	merged := make(map[string]string)

	// 1. Global strategy (only apply unattended defaults to agents unless overridden)
	globalJSON := h.getSetting("global_strategy_json", "{}")
	var globalMap map[string]string
	if err := json.Unmarshal([]byte(globalJSON), &globalMap); err == nil {
		if dev.ClientRole != "console" {
			for k, v := range globalMap {
				merged[k] = v
			}
		}
	}
	var maxTS int64 = 1
	if tsStr := h.getSetting("strategy_updated_at", "1"); tsStr != "" {
		if ts, err := strconv.ParseInt(tsStr, 10, 64); err == nil && ts > maxTS {
			maxTS = ts
		}
	}

	// 2. Group strategy
	if dev.GroupID > 0 {
		var grp model.DeviceGroup
		if err := h.DB.First(&grp, dev.GroupID).Error; err == nil {
			if grp.StrategyUpdatedAt > maxTS {
				maxTS = grp.StrategyUpdatedAt
			}
			var grpMap map[string]string
			if err := json.Unmarshal([]byte(grp.StrategyJSON), &grpMap); err == nil {
				for k, v := range grpMap {
					merged[k] = v
				}
			}
		}
	}

	// 3. Per-device override
	if dev.StrategyOverride != "" && dev.StrategyOverride != "{}" {
		var devMap map[string]string
		if err := json.Unmarshal([]byte(dev.StrategyOverride), &devMap); err == nil {
			for k, v := range devMap {
				merged[k] = v
			}
		}
	}

	return merged, maxTS
}

func parseAllowedGroups(raw string) []uint {
	if raw == "" {
		return nil
	}
	var ids []uint
	_ = json.Unmarshal([]byte(raw), &ids)
	return ids
}
