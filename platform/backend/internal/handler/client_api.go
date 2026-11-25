package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"polaris-platform/internal/auth"
	"polaris-platform/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type sysinfoReq struct {
	ID                   string `json:"id"`
	UUID                 string `json:"uuid"`
	Version              string `json:"version"`
	OS                   string `json:"os"`
	CPU                  string `json:"cpu"`
	Memory               string `json:"memory"`
	Hostname             string `json:"hostname"`
	Username             string `json:"username"`
	ClientRole           string `json:"client_role"`
	LocalIPs             string `json:"local_ips"`
	MacAddrs             string `json:"mac_addrs"`
	TempPassword         string `json:"temp_password"`
	HasPermanentPassword bool   `json:"has_permanent_password"`
	SchoolCode           string `json:"school_code"`
	EnrollToken          string `json:"enroll_token"`
	PresetNote           string `json:"preset-note"`
	DeviceName           string `json:"preset-device-name"`
	GroupName            string `json:"preset-device-group-name"`
	Alias                string `json:"preset-address-book-alias"`
}

func (h *Handler) HandleSysinfo(c *gin.Context) {
	var req sysinfoReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.ID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sysinfo payload"})
		return
	}

	role := normalizeRole(req.ClientRole)
	var dev model.Device
	err := h.DB.Where("id = ?", req.ID).First(&dev).Error
	isNew := err == gorm.ErrRecordNotFound

	var matchedSchool model.School
	schoolFound := false
	if tok := strings.TrimSpace(req.EnrollToken); tok != "" {
		if h.DB.Where("enroll_token = ? AND status = 1", tok).First(&matchedSchool).Error == nil {
			schoolFound = true
		}
	}
	if !schoolFound && strings.TrimSpace(req.SchoolCode) != "" {
		if h.DB.Where("LOWER(code) = ? AND status = 1", strings.ToLower(strings.TrimSpace(req.SchoolCode))).First(&matchedSchool).Error == nil {
			schoolFound = true
		}
	}

	if isNew {
		sid := uint(1)
		if schoolFound {
			sid = matchedSchool.ID
		}
		dev = model.Device{
			ID:         req.ID,
			SchoolID:   sid,
			GroupID:    h.defaultGroupIDForRoleInSchool(role, sid),
			IDAssigned: IsValidCustomID(req.ID),
		}
	} else if schoolFound && (dev.SchoolID == 0 || dev.SchoolID == 1) && matchedSchool.ID > 1 {
		dev.SchoolID = matchedSchool.ID
		dev.GroupID = h.defaultGroupIDForRoleInSchool(role, matchedSchool.ID)
	}
	if dev.SchoolID == 0 {
		dev.SchoolID = 1
	}

	dev.UUID = req.UUID
	if role != "" {
		dev.ClientRole = role
	} else if dev.ClientRole == "" {
		dev.ClientRole = "agent"
	}
	if req.DeviceName != "" {
		dev.Hostname = req.DeviceName
	} else if req.Hostname != "" {
		dev.Hostname = req.Hostname
	}
	if req.Alias != "" && dev.Alias == "" {
		dev.Alias = req.Alias
	}
	if req.OS != "" {
		dev.OS = req.OS
	}
	if req.CPU != "" {
		dev.CPU = req.CPU
	}
	if req.Memory != "" {
		dev.Memory = req.Memory
	}
	if req.Version != "" {
		dev.Version = req.Version
	}
	if req.Username != "" {
		dev.Username = req.Username
	}
	if req.LocalIPs != "" {
		dev.LocalIPs = req.LocalIPs
	}
	if req.MacAddrs != "" {
		dev.MacAddrs = req.MacAddrs
	}
	if req.TempPassword != "" {
		dev.TempPassword = req.TempPassword
	}
	dev.HasPermanentPassword = req.HasPermanentPassword
	if req.PresetNote != "" && dev.Note == "" {
		dev.Note = req.PresetNote
	}
	if req.GroupName != "" {
		var grp model.DeviceGroup
		if h.DB.Where("school_id = ? AND name = ?", dev.SchoolID, req.GroupName).First(&grp).Error == nil {
			dev.GroupID = grp.ID
		}
	}
	if dev.GroupID == 0 {
		dev.GroupID = h.defaultGroupIDForRoleInSchool(dev.ClientRole, dev.SchoolID)
	}
	dev.RemoteIP = c.ClientIP()
	dev.LastSeenAt = time.Now()

	if err := h.DB.Save(&dev).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Auto-assign school ID if enabled on this school and device doesn't have a school custom ID yet
	if dev.ClientRole != "console" && !dev.IDAssigned {
		var sch model.School
		if h.DB.First(&sch, dev.SchoolID).Error == nil && sch.AutoAssignID {
			var activeCmdCount int64
			h.DB.Model(&model.DeviceCommand{}).
				Where("device_id = ? AND command_type = ? AND status IN ?", dev.ID, "change_id", []string{"pending", "dispatched"}).
				Count(&activeCmdCount)
			if activeCmdCount == 0 {
				_ = h.DB.Transaction(func(tx *gorm.DB) error {
					var lockedSch model.School
					if err := tx.First(&lockedSch, sch.ID).Error; err != nil {
						return err
					}
					newID, err := h.nextSchoolIDTx(tx, &lockedSch)
					if err != nil {
						return err
					}
					_, err = h.queueDeviceCommand(tx, dev.ID, "change_id", map[string]string{
						"new_id": newID,
					}, "auto-enroll")
					return err
				})
			}
		}
	}
	c.String(http.StatusOK, "SYSINFO_UPDATED")
}

func (h *Handler) HandleSysinfoVer(c *gin.Context) {
	c.String(http.StatusOK, "1")
}

type heartbeatReq struct {
	ID                   string `json:"id"`
	UUID                 string `json:"uuid"`
	Ver                  int64  `json:"ver"`
	ModifiedAt           int64  `json:"modified_at"`
	Conns                []int  `json:"conns"`
	ClientRole           string `json:"client_role"`
	BoundUser            string `json:"bound_user"`
	LocalIPs             string `json:"local_ips"`
	TempPassword         string `json:"temp_password"`
	HasPermanentPassword *bool  `json:"has_permanent_password"`
}

func (h *Handler) HandleHeartbeat(c *gin.Context) {
	var req heartbeatReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.ID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid heartbeat"})
		return
	}

	var dev model.Device
	err := h.DB.Where("id = ?", req.ID).First(&dev).Error
	needSysinfo := false
	if err == gorm.ErrRecordNotFound {
		role := normalizeRole(req.ClientRole)
		if role == "" {
			role = "agent"
		}
		dev = model.Device{
			ID:         req.ID,
			UUID:       req.UUID,
			ClientRole: role,
			GroupID:    h.defaultGroupIDForRole(role),
			LastSeenAt: time.Now(),
			RemoteIP:   c.ClientIP(),
		}
		_ = h.DB.Create(&dev).Error
		needSysinfo = true
	}

	connsBytes, _ := json.Marshal(req.Conns)
	updates := map[string]interface{}{
		"last_seen_at":       time.Now(),
		"remote_ip":          c.ClientIP(),
		"active_conns_count": len(req.Conns),
		"active_conns_json":  string(connsBytes),
	}
	if req.UUID != "" && dev.UUID == "" {
		updates["uuid"] = req.UUID
	}
	if r := normalizeRole(req.ClientRole); r != "" && r != dev.ClientRole {
		updates["client_role"] = r
		dev.ClientRole = r
	}
	if req.BoundUser != "" {
		updates["bound_user"] = req.BoundUser
	}
	if req.LocalIPs != "" {
		updates["local_ips"] = req.LocalIPs
	}
	if req.TempPassword != "" {
		updates["temp_password"] = req.TempPassword
	}
	if req.HasPermanentPassword != nil {
		updates["has_permanent_password"] = *req.HasPermanentPassword
	}

	now := time.Now()
	if len(req.Conns) == 0 {
		h.DB.Model(&model.SessionRecord{}).
			Where("agent_id = ? AND status = ? AND last_active_at < ?", req.ID, "active", now.Add(-15*time.Second)).
			Updates(map[string]interface{}{"status": "closed", "ended_at": now})
	} else {
		h.DB.Model(&model.SessionRecord{}).
			Where("agent_id = ? AND status = ?", req.ID, "active").
			Update("last_active_at", now)
	}

	var disconnectIDs []int
	if dev.PendingDisconnect != "" {
		_ = json.Unmarshal([]byte(dev.PendingDisconnect), &disconnectIDs)
		updates["pending_disconnect"] = ""
	}
	h.DB.Model(&model.Device{}).Where("id = ?", req.ID).Updates(updates)

	strategyOpts, latestTS := h.resolveDeviceStrategy(&dev)
	resp := gin.H{
		"modified_at": latestTS,
	}
	if req.ModifiedAt != latestTS && len(strategyOpts) > 0 {
		resp["strategy"] = gin.H{
			"config_options": strategyOpts,
		}
	}
	if len(disconnectIDs) > 0 {
		resp["disconnect"] = disconnectIDs
	}
	if needSysinfo {
		resp["sysinfo"] = true
	}

	var pendingCmds []model.DeviceCommand
	h.DB.Where("device_id = ? AND (status = ? OR (status = ? AND dispatched_at < ?))",
		req.ID, "pending", "dispatched", now.Add(-20*time.Second)).
		Order("id ASC").Find(&pendingCmds)
	if len(pendingCmds) > 0 {
		cmdList := make([]gin.H, 0, len(pendingCmds))
		cmdIDs := make([]uint, 0, len(pendingCmds))
		for _, cmd := range pendingCmds {
			var payload map[string]interface{}
			_ = json.Unmarshal([]byte(cmd.PayloadJSON), &payload)
			if payload == nil {
				payload = map[string]interface{}{}
			}
			cmdList = append(cmdList, gin.H{
				"id":      cmd.ID,
				"type":    cmd.CommandType,
				"payload": payload,
			})
			cmdIDs = append(cmdIDs, cmd.ID)
		}
		h.DB.Model(&model.DeviceCommand{}).Where("id IN ?", cmdIDs).Updates(map[string]interface{}{
			"status":        "dispatched",
			"dispatched_at": now,
		})
		resp["commands"] = cmdList
	}
	c.JSON(http.StatusOK, resp)
}

type commandAckReq struct {
	CmdID                uint   `json:"cmd_id"`
	DeviceID             string `json:"device_id"`
	Status               string `json:"status"` // succeeded | failed
	Message              string `json:"message"`
	OldID                string `json:"old_id"`
	NewID                string `json:"new_id"`
	TempPassword         string `json:"temp_password"`
	HasPermanentPassword *bool  `json:"has_permanent_password"`
}

func (h *Handler) HandleCommandAck(c *gin.Context) {
	var req commandAckReq
	if err := c.ShouldBindJSON(&req); err != nil || req.CmdID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid command ack"})
		return
	}

	var cmd model.DeviceCommand
	if err := h.DB.First(&cmd, req.CmdID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "command not found"})
		return
	}

	now := time.Now()
	status := "failed"
	if req.Status == "succeeded" {
		status = "succeeded"
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&cmd).Updates(map[string]interface{}{
			"status":     status,
			"result_msg": req.Message,
			"acked_at":   now,
		}).Error; err != nil {
			return err
		}

		if status != "succeeded" {
			return nil
		}

		switch cmd.CommandType {
		case "change_id":
			oldID := firstNonEmpty(req.OldID, cmd.DeviceID)
			newID := strings.TrimSpace(req.NewID)
			if newID == "" {
				var p map[string]string
				_ = json.Unmarshal([]byte(cmd.PayloadJSON), &p)
				newID = strings.TrimSpace(p["new_id"])
			}
			if newID != "" && oldID != "" && newID != oldID {
				var oldDev model.Device
				if err := tx.Where("id = ?", oldID).First(&oldDev).Error; err == nil {
					var conflict model.Device
					if tx.Where("id = ?", newID).First(&conflict).Error == nil {
						tx.Delete(&conflict)
					}
					newDev := oldDev
					newDev.ID = newID
					newDev.IDAssigned = true
					newDev.LastSeenAt = now
					if req.TempPassword != "" {
						newDev.TempPassword = req.TempPassword
					}
					if err := tx.Create(&newDev).Error; err != nil {
						return err
					}
					tx.Delete(&model.Device{}, "id = ?", oldID)
				}
				tx.Model(&model.DeviceCommand{}).Where("device_id = ?", oldID).Update("device_id", newID)
				tx.Model(&model.SessionRecord{}).Where("agent_id = ?", oldID).Update("agent_id", newID)
				tx.Model(&model.SessionRecord{}).Where("console_id = ?", oldID).Update("console_id", newID)
				tx.Model(&model.AuditLog{}).Where("device_id = ?", oldID).Update("device_id", newID)
				tx.Model(&model.AuditLog{}).Where("peer_id = ?", oldID).Update("peer_id", newID)
			} else if newID != "" {
				tx.Model(&model.Device{}).Where("id = ?", newID).Update("id_assigned", true)
			}
		case "set_temp_password", "refresh_temp_password":
			if req.TempPassword != "" {
				tx.Model(&model.Device{}).Where("id = ?", cmd.DeviceID).Update("temp_password", req.TempPassword)
			}
		case "set_permanent_password":
			var p map[string]string
			_ = json.Unmarshal([]byte(cmd.PayloadJSON), &p)
			updates := map[string]interface{}{
				"has_permanent_password": true,
			}
			if pw := p["password"]; pw != "" {
				updates["managed_perm_pass"] = pw
			} else {
				updates["has_permanent_password"] = false
				updates["managed_perm_pass"] = ""
			}
			tx.Model(&model.Device{}).Where("id = ?", cmd.DeviceID).Updates(updates)
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) HandleAuditConn(c *gin.Context) {
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid audit"})
		return
	}

	nonce := stringVal(raw["nonce"])
	if nonce != "" {
		var existing model.AuditLog
		if h.DB.Where("nonce = ?", nonce).First(&existing).Error == nil {
			c.JSON(http.StatusOK, gin.H{"id": existing.ID})
			return
		}
	}

	deviceID := stringVal(raw["id"])
	connID := int64Val(raw["conn_id"])
	sessionID := int64Val(raw["session_id"])
	action := stringVal(raw["action"])
	clientIP := stringVal(raw["ip"])
	connType := int(int64Val(raw["type"]))

	var peerID, peerName string
	if peerArr, ok := raw["peer"].([]interface{}); ok {
		if len(peerArr) > 0 {
			peerID = stringVal(peerArr[0])
		}
		if len(peerArr) > 1 {
			peerName = stringVal(peerArr[1])
		}
		if action == "" {
			action = "auth"
		}
	}

	var operatorUser string
	if peerID != "" {
		var peerDev model.Device
		if h.DB.Where("id = ?", peerID).First(&peerDev).Error == nil {
			operatorUser = peerDev.BoundUser
		}
	}

	detailBytes, _ := json.Marshal(raw)
	entry := model.AuditLog{
		Nonce:        nonce,
		AuditType:    "conn",
		DeviceID:     deviceID,
		PeerID:       peerID,
		PeerName:     peerName,
		ConnID:       connID,
		SessionID:    sessionID,
		Action:       action,
		ConnType:     connType,
		ClientIP:     clientIP,
		OperatorUser: operatorUser,
		Detail:       string(detailBytes),
	}
	if entry.Nonce == "" {
		entry.Nonce = strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	_ = h.DB.Create(&entry).Error
	h.syncSessionFromAgentAudit(deviceID, connID, sessionID, peerID, peerName, operatorUser, clientIP, action, connType, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"id": entry.ID})
}

func (h *Handler) syncSessionFromAgentAudit(deviceID string, connID, sessionID int64, peerID, peerName, operatorUser, clientIP, action string, connType int, agentPublicIP string) {
	if deviceID == "" {
		return
	}
	now := time.Now()
	sessionKey := "agent:" + deviceID + ":" + strconv.FormatInt(connID, 10) + ":" + strconv.FormatInt(sessionID, 10)

	var agentDev model.Device
	h.DB.Where("id = ?", deviceID).First(&agentDev)
	agentName := agentDev.Alias
	if agentName == "" {
		agentName = agentDev.Hostname
	}
	agentRemoteIP := agentDev.RemoteIP
	if agentRemoteIP == "" {
		agentRemoteIP = agentPublicIP
	}

	if action == "close" {
		h.DB.Model(&model.SessionRecord{}).
			Where("session_key = ? OR (agent_id = ? AND status = ? AND conn_type = ?)", sessionKey, deviceID, "active", connType).
			Updates(map[string]interface{}{
				"status":         "closed",
				"ended_at":       now,
				"last_active_at": now,
			})
		return
	}

	var rec model.SessionRecord
	err := h.DB.Where("session_key = ?", sessionKey).First(&rec).Error
	if err == gorm.ErrRecordNotFound {
		rec = model.SessionRecord{
			SessionKey:    sessionKey,
			AgentID:       deviceID,
			AgentName:     agentName,
			AgentLocalIP:  agentDev.LocalIPs,
			AgentRemoteIP: agentRemoteIP,
			AgentOS:       agentDev.OS,
			AgentPassword: agentDev.TempPassword,
			ConnType:      connType,
			Status:        "active",
			Source:        "agent",
			StartedAt:     now,
			LastActiveAt:  now,
		}
	}

	if agentName != "" {
		rec.AgentName = agentName
	}
	if agentDev.LocalIPs != "" {
		rec.AgentLocalIP = agentDev.LocalIPs
	}
	if agentRemoteIP != "" {
		rec.AgentRemoteIP = agentRemoteIP
	}
	if agentDev.OS != "" {
		rec.AgentOS = agentDev.OS
	}
	if agentDev.TempPassword != "" {
		rec.AgentPassword = agentDev.TempPassword
	}
	if clientIP != "" && rec.ConsoleRemoteIP == "" {
		rec.ConsoleRemoteIP = clientIP
	}

	if peerID != "" {
		rec.ConsoleID = peerID
		rec.ConsoleName = peerName
		rec.ConnType = connType
		var consoleDev model.Device
		if h.DB.Where("id = ?", peerID).First(&consoleDev).Error == nil {
			if consoleDev.Alias != "" {
				rec.ConsoleName = consoleDev.Alias
			} else if consoleDev.Hostname != "" {
				rec.ConsoleName = consoleDev.Hostname
			}
			rec.ConsoleLocalIP = consoleDev.LocalIPs
			if consoleDev.RemoteIP != "" {
				rec.ConsoleRemoteIP = consoleDev.RemoteIP
			}
			rec.ConsoleOS = consoleDev.OS
			if consoleDev.BoundUser != "" {
				rec.ConsoleUser = consoleDev.BoundUser
			} else {
				rec.ConsoleUser = consoleDev.Username
			}
		}
		if operatorUser != "" {
			rec.ConsoleUser = operatorUser
		}
		// Deduplicate if Console already opened a matching session in the last 20 seconds
		var existingConsole model.SessionRecord
		if h.DB.Where("agent_id = ? AND console_id = ? AND status = ? AND source = ? AND started_at >= ?",
			deviceID, peerID, "active", "console", now.Add(-20*time.Second)).First(&existingConsole).Error == nil {
			rec.ConsoleLocalIP = firstNonEmpty(rec.ConsoleLocalIP, existingConsole.ConsoleLocalIP)
			rec.ConsoleRemoteIP = firstNonEmpty(rec.ConsoleRemoteIP, existingConsole.ConsoleRemoteIP)
			rec.ConsoleUser = firstNonEmpty(rec.ConsoleUser, existingConsole.ConsoleUser)
			rec.ConsoleOS = firstNonEmpty(rec.ConsoleOS, existingConsole.ConsoleOS)
			rec.Source = "both"
			h.DB.Delete(&existingConsole)
		}
	}
	rec.LastActiveAt = now
	_ = h.DB.Save(&rec).Error
}

type outgoingAuditReq struct {
	ID           string `json:"id"`
	UUID         string `json:"uuid"`
	ClientRole   string `json:"client_role"`
	Hostname     string `json:"hostname"`
	Username     string `json:"username"`
	BoundUser    string `json:"bound_user"`
	LocalIPs     string `json:"local_ips"`
	OS           string `json:"os"`
	Version      string `json:"version"`
	PeerID       string `json:"peer_id"`
	PeerName     string `json:"peer_name"`
	PeerPlatform string `json:"peer_platform"`
	PeerUsername string `json:"peer_username"`
	ConnType     int    `json:"conn_type"`
	Action       string `json:"action"`
	Nonce        string `json:"nonce"`
}

func (h *Handler) HandleAuditOutgoing(c *gin.Context) {
	var req outgoingAuditReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.ID) == "" || strings.TrimSpace(req.PeerID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid outgoing audit"})
		return
	}

	now := time.Now()
	consoleIP := c.ClientIP()
	role := normalizeRole(req.ClientRole)
	if role == "" {
		role = "console"
	}

	// Ensure Console device record is up to date
	var consoleDev model.Device
	if h.DB.Where("id = ?", req.ID).First(&consoleDev).Error == gorm.ErrRecordNotFound {
		consoleDev = model.Device{
			ID:         req.ID,
			UUID:       req.UUID,
			ClientRole: role,
			GroupID:    h.defaultGroupIDForRole(role),
		}
	}
	if req.Hostname != "" {
		consoleDev.Hostname = req.Hostname
	}
	if req.Username != "" {
		consoleDev.Username = req.Username
	}
	if req.BoundUser != "" {
		consoleDev.BoundUser = req.BoundUser
	}
	if req.LocalIPs != "" {
		consoleDev.LocalIPs = req.LocalIPs
	}
	if req.OS != "" {
		consoleDev.OS = req.OS
	}
	if req.Version != "" {
		consoleDev.Version = req.Version
	}
	consoleDev.RemoteIP = consoleIP
	consoleDev.LastSeenAt = now
	_ = h.DB.Save(&consoleDev).Error

	sessionKey := "outgoing:" + req.ID + ":" + req.Nonce
	if req.Action == "close" {
		h.DB.Model(&model.SessionRecord{}).
			Where("session_key = ? OR (agent_id = ? AND console_id = ? AND status = ?)", sessionKey, req.PeerID, req.ID, "active").
			Updates(map[string]interface{}{
				"status":         "closed",
				"ended_at":       now,
				"last_active_at": now,
			})
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	var agentDev model.Device
	agentExists := h.DB.Where("id = ?", req.PeerID).First(&agentDev).Error == nil
	if !agentExists {
		agentDev = model.Device{
			ID:         req.PeerID,
			ClientRole: "agent",
			Hostname:   req.PeerName,
			Username:   req.PeerUsername,
			OS:         req.PeerPlatform,
			GroupID:    1,
			LastSeenAt: now,
		}
		_ = h.DB.Create(&agentDev).Error
	}

	agentName := firstNonEmpty(agentDev.Alias, agentDev.Hostname, req.PeerName, req.PeerID)
	consoleName := firstNonEmpty(consoleDev.Alias, consoleDev.Hostname, req.Hostname, req.ID)
	consoleUser := firstNonEmpty(req.BoundUser, consoleDev.BoundUser, req.Username, consoleDev.Username)

	// Check if Agent already created an active SessionRecord in the last 20s
	var existing model.SessionRecord
	if h.DB.Where("agent_id = ? AND (console_id = ? OR console_id = '') AND status = ? AND started_at >= ?",
		req.PeerID, req.ID, "active", now.Add(-20*time.Second)).Order("id DESC").First(&existing).Error == nil {
		existing.ConsoleID = req.ID
		existing.ConsoleName = consoleName
		existing.ConsoleLocalIP = firstNonEmpty(req.LocalIPs, consoleDev.LocalIPs)
		existing.ConsoleRemoteIP = consoleIP
		existing.ConsoleUser = consoleUser
		existing.ConsoleOS = firstNonEmpty(req.OS, consoleDev.OS)
		existing.AgentName = firstNonEmpty(existing.AgentName, agentName)
		existing.AgentLocalIP = firstNonEmpty(existing.AgentLocalIP, agentDev.LocalIPs)
		existing.AgentRemoteIP = firstNonEmpty(existing.AgentRemoteIP, agentDev.RemoteIP)
		existing.AgentPassword = firstNonEmpty(existing.AgentPassword, agentDev.TempPassword)
		existing.Source = "both"
		existing.LastActiveAt = now
		_ = h.DB.Save(&existing).Error
	} else {
		rec := model.SessionRecord{
			SessionKey:      sessionKey,
			AgentID:         req.PeerID,
			AgentName:       agentName,
			AgentLocalIP:    agentDev.LocalIPs,
			AgentRemoteIP:   agentDev.RemoteIP,
			AgentOS:         firstNonEmpty(agentDev.OS, req.PeerPlatform),
			AgentPassword:   agentDev.TempPassword,
			ConsoleID:       req.ID,
			ConsoleName:     consoleName,
			ConsoleLocalIP:  firstNonEmpty(req.LocalIPs, consoleDev.LocalIPs),
			ConsoleRemoteIP: consoleIP,
			ConsoleUser:     consoleUser,
			ConsoleOS:       firstNonEmpty(req.OS, consoleDev.OS),
			ConnType:        req.ConnType,
			Status:          "active",
			Source:          "console",
			StartedAt:       now,
			LastActiveAt:    now,
		}
		_ = h.DB.Create(&rec).Error
	}

	auditNonce := "out-" + req.Nonce
	detailBytes, _ := json.Marshal(req)
	_ = h.DB.FirstOrCreate(&model.AuditLog{
		Nonce:        auditNonce,
		AuditType:    "conn",
		DeviceID:     req.PeerID,
		PeerID:       req.ID,
		PeerName:     consoleName,
		Action:       "connect",
		ConnType:     req.ConnType,
		ClientIP:     consoleIP,
		OperatorUser: consoleUser,
		Detail:       string(detailBytes),
	}, model.AuditLog{Nonce: auditNonce}).Error

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (h *Handler) HandleAuditFile(c *gin.Context) {
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file audit"})
		return
	}
	nonce := stringVal(raw["nonce"])
	if nonce == "" {
		nonce = "file-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	detailBytes, _ := json.Marshal(raw)
	entry := model.AuditLog{
		Nonce:     nonce,
		AuditType: "file",
		DeviceID:  stringVal(raw["id"]),
		PeerID:    stringVal(raw["peer_id"]),
		ConnID:    int64Val(raw["conn_id"]),
		Action:    "file",
		ConnType:  1,
		Detail:    string(detailBytes),
	}
	_ = h.DB.FirstOrCreate(&entry, model.AuditLog{Nonce: nonce}).Error
	c.JSON(http.StatusOK, gin.H{"id": entry.ID})
}

func (h *Handler) HandleAuditAlarm(c *gin.Context) {
	var raw map[string]interface{}
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid alarm audit"})
		return
	}
	nonce := stringVal(raw["nonce"])
	if nonce == "" {
		nonce = "alarm-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	detailBytes, _ := json.Marshal(raw)
	entry := model.AuditLog{
		Nonce:     nonce,
		AuditType: "alarm",
		DeviceID:  stringVal(raw["id"]),
		ConnID:    int64Val(raw["conn_id"]),
		Action:    "alarm",
		Detail:    string(detailBytes),
	}
	_ = h.DB.FirstOrCreate(&entry, model.AuditLog{Nonce: nonce}).Error
	c.JSON(http.StatusOK, gin.H{"id": entry.ID})
}

func (h *Handler) HandleUpdateAuditNote(c *gin.Context) {
	var req struct {
		GUID string `json:"guid"`
		Note string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.GUID != "" {
		h.DB.Model(&model.AuditLog{}).Where("nonce = ?", req.GUID).Update("note", req.Note)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) HandleLoginOptions(c *gin.Context) {
	c.JSON(http.StatusOK, []string{})
}

type clientLoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	ID       string `json:"id"`
	UUID     string `json:"uuid"`
}

func (h *Handler) HandleClientLogin(c *gin.Context) {
	var req clientLoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误"})
		return
	}
	var user model.User
	if err := h.DB.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"error": "账号或密码错误"})
		return
	}
	if user.Status != 1 {
		c.JSON(http.StatusOK, gin.H{"error": "该账号已被停用"})
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusOK, gin.H{"error": "账号或密码错误"})
		return
	}

	token, err := auth.GenerateToken(h.JWTSecret, &user, 30*24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "签发令牌失败"})
		return
	}

	if req.ID != "" {
		h.DB.Model(&model.Device{}).Where("id = ?", req.ID).Updates(map[string]interface{}{
			"bound_user": user.Username,
			"last_seen_at": time.Now(),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": token,
		"type":         "access_token",
		"user":         toClientUserPayload(&user),
	})
}

func (h *Handler) HandleCurrentUser(c *gin.Context) {
	u := c.MustGet("user").(*model.User)
	var req struct {
		ID   string `json:"id"`
		UUID string `json:"uuid"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.ID != "" {
		h.DB.Model(&model.Device{}).Where("id = ?", req.ID).Update("bound_user", u.Username)
	}
	c.JSON(http.StatusOK, toClientUserPayload(u))
}

func (h *Handler) HandleClientLogout(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.ID != "" {
		h.DB.Model(&model.Device{}).Where("id = ?", req.ID).Update("bound_user", "")
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) HandleAccessibleGroups(c *gin.Context) {
	u := c.MustGet("user").(*model.User)
	var groups []model.DeviceGroup
	query := h.DB.Order("id ASC")
	if !u.IsAdmin && u.Role != "admin" {
		allowed := parseAllowedGroups(u.AllowedGroups)
		if len(allowed) > 0 {
			query = query.Where("id IN ?", allowed)
		}
	}
	query.Find(&groups)
	data := make([]gin.H, 0, len(groups))
	for _, g := range groups {
		data = append(data, gin.H{
			"name": g.Name,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"total": len(data),
		"data":  data,
	})
}

func (h *Handler) HandleClientUsers(c *gin.Context) {
	var users []model.User
	h.DB.Where("status = 1").Order("id ASC").Find(&users)
	data := make([]gin.H, 0, len(users))
	for i := range users {
		data = append(data, toClientUserPayload(&users[i]))
	}
	c.JSON(http.StatusOK, gin.H{
		"total": len(data),
		"data":  data,
	})
}

func (h *Handler) HandleClientPeers(c *gin.Context) {
	u := c.MustGet("user").(*model.User)
	var groups []model.DeviceGroup
	h.DB.Find(&groups)
	groupMap := make(map[uint]string)
	for _, g := range groups {
		groupMap[g.ID] = g.Name
	}

	query := h.DB.Where("client_role != ?", "console").Order("last_seen_at DESC")
	if !u.IsAdmin && u.Role != "admin" {
		allowed := parseAllowedGroups(u.AllowedGroups)
		if len(allowed) > 0 {
			query = query.Where("group_id IN ?", allowed)
		}
	}

	var devices []model.Device
	query.Find(&devices)

	data := make([]gin.H, 0, len(devices))
	for _, d := range devices {
		name := d.Alias
		if name == "" {
			name = d.Hostname
		}
		if name == "" {
			name = d.ID
		}
		status := 0
		if h.isDeviceOnline(d.LastSeenAt) {
			status = 1
		}
		data = append(data, gin.H{
			"id":                d.ID,
			"status":            status,
			"user":              d.BoundUser,
			"user_name":         d.BoundUser,
			"device_group_name": groupMap[d.GroupID],
			"note":              d.Note,
			"info": gin.H{
				"username":    d.Username,
				"os":          d.OS,
				"device_name": name,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total": len(data),
		"data":  data,
	})
}

func (h *Handler) HandleGetAB(c *gin.Context) {
	u := c.MustGet("user").(*model.User)
	var ab model.PersonalAddressBook
	if err := h.DB.Where("user_id = ?", u.ID).First(&ab).Error; err != nil || ab.Data == "" {
		c.String(http.StatusOK, "null")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": ab.Data,
	})
}

func (h *Handler) HandleSaveAB(c *gin.Context) {
	u := c.MustGet("user").(*model.User)
	var req struct {
		Data string `json:"data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ab payload"})
		return
	}
	ab := model.PersonalAddressBook{
		UserID: u.ID,
		Data:   req.Data,
	}
	h.DB.Save(&ab)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) HandleABSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"max_peer_one_ab": 1000})
}

func (h *Handler) HandleABPersonal(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"guid": ""})
}

func (h *Handler) HandleABSharedProfiles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"total": 0, "data": []interface{}{}})
}

func (h *Handler) defaultGroupIDForRole(role string) uint {
	return h.defaultGroupIDForRoleInSchool(role, 1)
}

func (h *Handler) defaultGroupIDForRoleInSchool(role string, schoolID uint) uint {
	if schoolID == 0 {
		schoolID = 1
	}
	if role == "console" {
		var g model.DeviceGroup
		if h.DB.Where("school_id = ? AND name = ?", schoolID, "教师控制台").First(&g).Error == nil {
			return g.ID
		}
	}
	return h.ensureSchoolDefaultGroup(h.DB, schoolID)
}

func toClientUserPayload(u *model.User) gin.H {
	return gin.H{
		"name":         u.Username,
		"display_name": u.DisplayName,
		"email":        u.Email,
		"avatar":       "",
		"note":         u.Note,
		"status":       u.Status,
		"is_admin":     u.IsAdmin,
		"info":         gin.H{},
	}
}

func normalizeRole(role string) string {
	r := strings.ToLower(strings.TrimSpace(role))
	switch r {
	case "agent", "incoming":
		return "agent"
	case "console", "outgoing":
		return "console"
	default:
		return ""
	}
}

func stringVal(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func int64Val(v interface{}) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case string:
		p, _ := strconv.ParseInt(n, 10, 64)
		return p
	default:
		return 0
	}
}
