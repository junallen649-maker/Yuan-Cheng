package model

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type School struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	Name                string    `gorm:"uniqueIndex;size:128" json:"name"`
	Code                string    `gorm:"uniqueIndex;size:32" json:"code"`
	EnrollToken         string    `gorm:"uniqueIndex;size:64" json:"enroll_token"`
	IDSeq               uint      `gorm:"default:0" json:"id_seq"`
	AutoAssignID        bool      `gorm:"default:true" json:"auto_assign_id"`
	DefaultStrategyJSON string    `gorm:"type:text" json:"default_strategy_json"`
	Status              int       `gorm:"default:1" json:"status"`
	Contact             string    `gorm:"size:128" json:"contact"`
	Note                string    `gorm:"size:512" json:"note"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type Device struct {
	ID                   string    `gorm:"primaryKey;size:64" json:"id"`
	UUID                 string    `gorm:"index;size:128" json:"uuid"`
	ClientRole           string    `gorm:"index;size:32;default:agent" json:"client_role"` // agent | console
	Hostname             string    `gorm:"size:128" json:"hostname"`
	Alias                string    `gorm:"size:128" json:"alias"`
	OS                   string    `gorm:"size:128" json:"os"`
	CPU                  string    `gorm:"size:128" json:"cpu"`
	Memory               string    `gorm:"size:64" json:"memory"`
	Version              string    `gorm:"size:64" json:"version"`
	Username             string    `gorm:"size:128" json:"username"`
	LocalIPs             string    `gorm:"size:512" json:"local_ips"`
	RemoteIP             string    `gorm:"size:64" json:"remote_ip"`
	MacAddrs             string    `gorm:"size:512" json:"mac_addrs"`
	BoundUser            string    `gorm:"index;size:128" json:"bound_user"`
	TempPassword         string    `gorm:"size:64" json:"temp_password"`
	HasPermanentPassword bool      `gorm:"default:false" json:"has_permanent_password"`
	ManagedPermPass      string    `gorm:"size:128" json:"managed_perm_pass"`
	SchoolID             uint      `gorm:"index;default:1" json:"school_id"`
	GroupID              uint      `gorm:"index;default:1" json:"group_id"`
	IDAssigned           bool      `gorm:"default:false" json:"id_assigned"`
	Note                 string    `gorm:"type:text" json:"note"`
	StrategyOverride     string    `gorm:"type:text" json:"strategy_override"`
	ActiveConnsCount     int       `gorm:"default:0" json:"active_conns_count"`
	ActiveConnsJSON      string    `gorm:"type:text" json:"active_conns_json"`
	PendingDisconnect    string    `gorm:"type:text" json:"-"`
	LastSeenAt           time.Time `gorm:"index" json:"last_seen_at"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`

	// Computed fields for JSON response
	Online         bool   `gorm:"-" json:"online"`
	SchoolName     string `gorm:"-" json:"school_name"`
	GroupName      string `gorm:"-" json:"group_name"`
	UnattendedPass string `gorm:"-" json:"unattended_pass"`
	LatestCmdInfo  string `gorm:"-" json:"latest_cmd_info"`
}

type DeviceGroup struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	SchoolID          uint      `gorm:"index;default:1" json:"school_id"`
	Name              string    `gorm:"size:128" json:"name"`
	Description       string    `gorm:"size:512" json:"description"`
	StrategyJSON      string    `gorm:"type:text" json:"strategy_json"`
	UnattendedPass    string    `gorm:"size:128" json:"unattended_pass"`
	StrategyUpdatedAt int64     `json:"strategy_updated_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type User struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Username      string    `gorm:"uniqueIndex;size:128" json:"username"`
	DisplayName   string    `gorm:"size:128" json:"display_name"`
	Email         string    `gorm:"size:128" json:"email"`
	PasswordHash  string    `gorm:"size:255" json:"-"`
	Role          string    `gorm:"size:32;default:teacher" json:"role"` // admin | teacher | viewer
	IsAdmin       bool      `gorm:"default:false" json:"is_admin"`
	SchoolID      uint      `gorm:"index;default:0" json:"school_id"` // 0 = 平台超管; >0 = 所属学校
	Status        int       `gorm:"default:1" json:"status"`          // 1: normal, 0: disabled
	AllowedGroups string    `gorm:"type:text" json:"allowed_groups"`
	Note          string    `gorm:"size:512" json:"note"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type DeviceCommand struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	DeviceID     string     `gorm:"index;size:64" json:"device_id"`
	CommandType  string     `gorm:"size:64" json:"command_type"` // change_id | set_temp_password | refresh_temp_password | set_permanent_password
	PayloadJSON  string     `gorm:"type:text" json:"payload_json"`
	Status       string     `gorm:"index;size:32;default:pending" json:"status"` // pending | dispatched | succeeded | failed
	ResultMsg    string     `gorm:"size:512" json:"result_msg"`
	CreatedBy    string     `gorm:"size:128" json:"created_by"`
	DispatchedAt *time.Time `json:"dispatched_at"`
	AckedAt      *time.Time `json:"acked_at"`
	CreatedAt    time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type AuditLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Nonce        string    `gorm:"uniqueIndex;size:128" json:"nonce"`
	AuditType    string    `gorm:"index;size:32" json:"audit_type"` // conn | file | alarm
	DeviceID     string    `gorm:"index;size:64" json:"device_id"`
	PeerID       string    `gorm:"index;size:64" json:"peer_id"`
	PeerName     string    `gorm:"size:128" json:"peer_name"`
	ConnID       int64     `gorm:"index" json:"conn_id"`
	SessionID    int64     `json:"session_id"`
	Action       string    `gorm:"size:64" json:"action"`
	ConnType     int       `json:"conn_type"`
	ClientIP     string    `gorm:"size:64" json:"client_ip"`
	OperatorUser string    `gorm:"size:128" json:"operator_user"`
	Detail       string    `gorm:"type:text" json:"detail"`
	Note         string    `gorm:"type:text" json:"note"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}

type SessionRecord struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	SessionKey      string     `gorm:"uniqueIndex;size:191" json:"session_key"`
	AgentID         string     `gorm:"index;size:64" json:"agent_id"`
	AgentName       string     `gorm:"size:128" json:"agent_name"`
	AgentLocalIP    string     `gorm:"size:512" json:"agent_local_ip"`
	AgentRemoteIP   string     `gorm:"size:64" json:"agent_remote_ip"`
	AgentOS         string     `gorm:"size:128" json:"agent_os"`
	AgentPassword   string     `gorm:"size:64" json:"agent_password"`
	ConsoleID       string     `gorm:"index;size:64" json:"console_id"`
	ConsoleName     string     `gorm:"size:128" json:"console_name"`
	ConsoleLocalIP  string     `gorm:"size:512" json:"console_local_ip"`
	ConsoleRemoteIP string     `gorm:"size:64" json:"console_remote_ip"`
	ConsoleUser     string     `gorm:"size:128" json:"console_user"`
	ConsoleOS       string     `gorm:"size:128" json:"console_os"`
	ConnType        int        `json:"conn_type"`
	Status          string     `gorm:"index;size:32" json:"status"` // active | closed
	Source          string     `gorm:"size:32" json:"source"`       // agent | console | both
	StartedAt       time.Time  `gorm:"index" json:"started_at"`
	EndedAt         *time.Time `json:"ended_at"`
	LastActiveAt    time.Time  `gorm:"index" json:"last_active_at"`
}

type PersonalAddressBook struct {
	UserID    uint      `gorm:"primaryKey" json:"user_id"`
	Data      string    `gorm:"type:text" json:"data"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Setting struct {
	Key       string    `gorm:"primaryKey;size:128" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

func InitDB(dbPath, defaultAdminUser, defaultAdminPass, envJWTSecret string) (*gorm.DB, string, error) {
	if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, "", err
		}
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, "", err
	}

	if err := db.AutoMigrate(&School{}, &Device{}, &DeviceGroup{}, &User{}, &DeviceCommand{}, &AuditLog{}, &SessionRecord{}, &PersonalAddressBook{}, &Setting{}); err != nil {
		return nil, "", err
	}

	// Replace legacy single-column unique index on device_groups.name with per-school unique index.
	db.Exec("DROP INDEX IF EXISTS idx_device_groups_name")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_school_group_name ON device_groups(school_id, name)")

	if err := seedDefaults(db, defaultAdminUser, defaultAdminPass); err != nil {
		return nil, "", err
	}

	jwtSecret, err := ensureJWTSecret(db, envJWTSecret)
	if err != nil {
		return nil, "", err
	}

	return db, jwtSecret, nil
}

func seedDefaults(db *gorm.DB, adminUser, adminPass string) error {
	var schoolCount int64
	db.Model(&School{}).Count(&schoolCount)
	if schoolCount == 0 {
		defaultStrat, _ := json.Marshal(map[string]string{
			"approve-mode":        "password",
			"verification-method": "use-both-passwords",
		})
		school := School{
			ID:                  1,
			Name:                "默认学校",
			Code:                "sch01",
			EnrollToken:         "sch01-enroll",
			IDSeq:               0,
			AutoAssignID:        false,
			DefaultStrategyJSON: string(defaultStrat),
			Status:              1,
			Note:                "系统内置默认学校（未指定学校代码的终端默认归属此处）",
		}
		if err := db.FirstOrCreate(&school, School{ID: 1}).Error; err != nil {
			return err
		}

		var groupCount int64
		db.Model(&DeviceGroup{}).Count(&groupCount)
		if groupCount == 0 {
			defaultStrategy, _ := json.Marshal(map[string]string{
				"approve-mode":        "password",
				"verification-method": "use-both-passwords",
			})
			groups := []DeviceGroup{
				{
					ID:                1,
					SchoolID:          1,
					Name:              "默认机房",
					Description:       "新接入的学校终端默认归属分组",
					StrategyJSON:      string(defaultStrategy),
					StrategyUpdatedAt: time.Now().Unix(),
				},
				{
					ID:                2,
					SchoolID:          1,
					Name:              "教师控制台",
					Description:       "教师与运维人员使用的主控终端分组",
					StrategyJSON:      "{}",
					StrategyUpdatedAt: time.Now().Unix(),
				},
			}
			for _, g := range groups {
				if err := db.FirstOrCreate(&g, DeviceGroup{ID: g.ID}).Error; err != nil {
					return err
				}
			}
		}
	}

	// Backfill legacy rows with school_id = 0
	db.Model(&DeviceGroup{}).Where("school_id = 0 OR school_id IS NULL").Update("school_id", 1)
	db.Model(&Device{}).Where("school_id = 0 OR school_id IS NULL").Update("school_id", 1)

	var userCount int64
	db.Model(&User{}).Count(&userCount)
	if userCount == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		admin := User{
			Username:      adminUser,
			DisplayName:   "平台超级管理员",
			Email:         "admin@easyreach.asia",
			PasswordHash:  string(hash),
			Role:          "admin",
			IsAdmin:       true,
			SchoolID:      0,
			Status:        1,
			AllowedGroups: "[]",
			Note:          "默认平台超级管理员账号，可管理全部学校，请及时修改密码",
		}
		if err := db.Create(&admin).Error; err != nil {
			return err
		}
	}

	defaultSettings := map[string]string{
		"school_name":           "Polaris 多校远控运维平台",
		"global_strategy_json":  `{"approve-mode":"password","verification-method":"use-both-passwords"}`,
		"strategy_updated_at":   "1",
		"heartbeat_timeout_sec": "45",
	}
	for k, v := range defaultSettings {
		var existing Setting
		if err := db.Where("key = ?", k).First(&existing).Error; err == gorm.ErrRecordNotFound {
			if err := db.Create(&Setting{Key: k, Value: v}).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func ensureJWTSecret(db *gorm.DB, envSecret string) (string, error) {
	if envSecret != "" {
		return envSecret, nil
	}
	var setting Setting
	err := db.Where("key = ?", "jwt_secret").First(&setting).Error
	if err == nil && setting.Value != "" {
		return setting.Value, nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	secret := hex.EncodeToString(buf)
	if err := db.Save(&Setting{Key: "jwt_secret", Value: secret}).Error; err != nil {
		return "", err
	}
	return secret, nil
}
