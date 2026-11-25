package handler

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"polaris-platform/internal/config"
	"polaris-platform/internal/model"

	"gorm.io/gorm"
)

var customIDRegex = regexp.MustCompile(`^[a-zA-Z][\w-]{5,15}$`)
var schoolCodeRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]{1,7}$`)

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

func IsValidCustomID(id string) bool {
	return customIDRegex.MatchString(strings.TrimSpace(id))
}

func IsValidSchoolCode(code string) bool {
	return schoolCodeRegex.MatchString(strings.TrimSpace(code))
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

	// 1. Global strategy
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

	// 2. School default strategy
	if dev.SchoolID > 0 && dev.ClientRole != "console" {
		var sch model.School
		if h.DB.First(&sch, dev.SchoolID).Error == nil && sch.DefaultStrategyJSON != "" {
			var schMap map[string]string
			if err := json.Unmarshal([]byte(sch.DefaultStrategyJSON), &schMap); err == nil {
				for k, v := range schMap {
					merged[k] = v
				}
			}
			if ts := sch.UpdatedAt.Unix(); ts > maxTS {
				maxTS = ts
			}
		}
	}

	// 3. Group strategy
	if dev.GroupID > 0 {
		var grp model.DeviceGroup
		if h.DB.First(&grp, dev.GroupID).Error == nil {
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

	// 4. Per-device override
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

func (h *Handler) nextSchoolIDTx(tx *gorm.DB, school *model.School) (string, error) {
	prefix := strings.ToLower(strings.TrimSpace(school.Code))
	if !IsValidSchoolCode(prefix) {
		prefix = fmt.Sprintf("sc%02d", school.ID)
	}
	for attempt := 0; attempt < 500; attempt++ {
		school.IDSeq++
		candidate := fmt.Sprintf("%s-%04d", prefix, school.IDSeq)
		if len(candidate) > 16 {
			candidate = fmt.Sprintf("%s-%d", prefix, school.IDSeq)
		}
		if !IsValidCustomID(candidate) {
			continue
		}
		var count int64
		tx.Model(&model.Device{}).Where("id = ?", candidate).Count(&count)
		if count == 0 {
			var pendingCount int64
			tx.Model(&model.DeviceCommand{}).
				Where("command_type = ? AND status IN ? AND payload_json LIKE ?", "change_id", []string{"pending", "dispatched"}, "%\""+candidate+"\"%").
				Count(&pendingCount)
			if pendingCount == 0 {
				if err := tx.Model(school).Update("id_seq", school.IDSeq).Error; err != nil {
					return "", err
				}
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("无法为学校 %s 生成可用连接代码", school.Name)
}

func (h *Handler) queueDeviceCommand(tx *gorm.DB, deviceID, cmdType string, payload map[string]string, createdBy string) (*model.DeviceCommand, error) {
	b, _ := json.Marshal(payload)
	// Cancel older pending/dispatched commands of the exact same type to avoid stale overwrites
	tx.Model(&model.DeviceCommand{}).
		Where("device_id = ? AND command_type = ? AND status IN ?", deviceID, cmdType, []string{"pending", "dispatched"}).
		Updates(map[string]interface{}{"status": "failed", "result_msg": "已被新指令覆盖"})

	cmd := &model.DeviceCommand{
		DeviceID:    deviceID,
		CommandType: cmdType,
		PayloadJSON: string(b),
		Status:      "pending",
		CreatedBy:   createdBy,
	}
	if err := tx.Create(cmd).Error; err != nil {
		return nil, err
	}
	return cmd, nil
}

func (h *Handler) ensureSchoolDefaultGroup(tx *gorm.DB, schoolID uint) uint {
	var grp model.DeviceGroup
	if err := tx.Where("school_id = ?", schoolID).Order("id ASC").First(&grp).Error; err == nil {
		return grp.ID
	}
	defaultStrat, _ := json.Marshal(map[string]string{
		"approve-mode":        "password",
		"verification-method": "use-both-passwords",
	})
	grp = model.DeviceGroup{
		SchoolID:          schoolID,
		Name:              "默认机房",
		Description:       "学校默认机房分组",
		StrategyJSON:      string(defaultStrat),
		StrategyUpdatedAt: time.Now().Unix(),
	}
	if err := tx.Create(&grp).Error; err != nil {
		return 1
	}
	return grp.ID
}

func parseAllowedGroups(raw string) []uint {
	if raw == "" {
		return nil
	}
	var ids []uint
	_ = json.Unmarshal([]byte(raw), &ids)
	return ids
}

func isSuperAdmin(u *model.User) bool {
	return u != nil && u.SchoolID == 0 && (u.IsAdmin || u.Role == "admin")
}
