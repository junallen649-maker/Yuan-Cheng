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
)

func (h *Handler) HandleAdminLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名和密码"})
		return
	}
	var user model.User
	if err := h.DB.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	if user.Status != 1 {
		c.JSON(http.StatusForbidden, gin.H{"error": "账号已被禁用"})
		return
	}
	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	token, err := auth.GenerateToken(h.JWTSecret, &user, 7*24*time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "登录令牌生成失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}

func (h *Handler) HandleAdminMe(c *gin.Context) {
	u := c.MustGet("user").(*model.User)
	c.JSON(http.StatusOK, u)
}

func (h *Handler) HandleChangePassword(c *gin.Context) {
	u := c.MustGet("user").(*model.User)
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.NewPassword) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "新密码长度不能少于 6 位"})
		return
	}
	if !auth.CheckPassword(u.PasswordHash, req.OldPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "原密码不正确"})
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}
	h.DB.Model(u).Update("password_hash", hash)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) HandleOverview(c *gin.Context) {
	var devices []model.Device
	h.DB.Find(&devices)

	var groups []model.DeviceGroup
	h.DB.Find(&groups)
	groupNames := make(map[uint]string)
	groupPasses := make(map[uint]string)
	for _, g := range groups {
		groupNames[g.ID] = g.Name
		groupPasses[g.ID] = g.UnattendedPass
	}

	var totalAgents, onlineAgents, totalConsoles, onlineConsoles, activeConns int
	groupDist := make(map[string]map[string]int)

	for _, d := range devices {
		online := h.isDeviceOnline(d.LastSeenAt)
		gName := groupNames[d.GroupID]
		if gName == "" {
			gName = "未分组"
		}
		if _, ok := groupDist[gName]; !ok {
			groupDist[gName] = map[string]int{"total": 0, "online": 0}
		}
		groupDist[gName]["total"]++
		if online {
			groupDist[gName]["online"]++
		}

		if d.ClientRole == "console" {
			totalConsoles++
			if online {
				onlineConsoles++
			}
		} else {
			totalAgents++
			if online {
				onlineAgents++
				activeConns += d.ActiveConnsCount
			}
		}
	}

	var activeSessionRows int64
	h.DB.Model(&model.SessionRecord{}).Where("status = ?", "active").Count(&activeSessionRows)
	if int(activeSessionRows) > activeConns {
		activeConns = int(activeSessionRows)
	}

	todayStart := time.Now().Truncate(24 * time.Hour)
	var todayAudits int64
	h.DB.Model(&model.AuditLog{}).Where("created_at >= ?", todayStart).Count(&todayAudits)

	var totalUsers int64
	h.DB.Model(&model.User{}).Where("status = 1").Count(&totalUsers)

	var recentAudits []model.AuditLog
	h.DB.Order("created_at DESC").Limit(8).Find(&recentAudits)

	c.JSON(http.StatusOK, gin.H{
		"school_name":     h.getSetting("school_name", "校园远控运维平台"),
		"server_host":     h.Cfg.ServerPublicHost,
		"rendezvous_port": h.Cfg.RendezvousPort,
		"relay_port":      h.Cfg.RelayPort,
		"total_devices":   len(devices),
		"total_agents":    totalAgents,
		"online_agents":   onlineAgents,
		"total_consoles":  totalConsoles,
		"online_consoles": onlineConsoles,
		"active_sessions": activeConns,
		"today_audits":    todayAudits,
		"total_users":     totalUsers,
		"group_stats":     groupDist,
		"recent_audits":   recentAudits,
	})
}

func (h *Handler) HandleListDevices(c *gin.Context) {
	role := strings.TrimSpace(c.Query("role"))
	status := strings.TrimSpace(c.Query("status"))
	groupIDStr := strings.TrimSpace(c.Query("group_id"))
	keyword := strings.TrimSpace(c.Query("keyword"))

	var groups []model.DeviceGroup
	h.DB.Find(&groups)
	groupNames := make(map[uint]string)
	groupPasses := make(map[uint]string)
	for _, g := range groups {
		groupNames[g.ID] = g.Name
		groupPasses[g.ID] = g.UnattendedPass
	}

	query := h.DB.Model(&model.Device{}).Order("last_seen_at DESC")
	if role != "" && role != "all" {
		query = query.Where("client_role = ?", role)
	}
	if groupIDStr != "" && groupIDStr != "0" {
		if gid, err := strconv.Atoi(groupIDStr); err == nil {
			query = query.Where("group_id = ?", gid)
		}
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("id LIKE ? OR hostname LIKE ? OR alias LIKE ? OR local_ips LIKE ? OR remote_ip LIKE ? OR username LIKE ? OR bound_user LIKE ?",
			like, like, like, like, like, like, like)
	}

	var devices []model.Device
	query.Find(&devices)

	filtered := make([]model.Device, 0, len(devices))
	for _, d := range devices {
		d.Online = h.isDeviceOnline(d.LastSeenAt)
		d.GroupName = groupNames[d.GroupID]
		d.UnattendedPass = groupPasses[d.GroupID]
		if status == "online" && !d.Online {
			continue
		}
		if status == "offline" && d.Online {
			continue
		}
		filtered = append(filtered, d)
	}

	c.JSON(http.StatusOK, gin.H{
		"total": len(filtered),
		"items": filtered,
	})
}

func (h *Handler) HandleUpdateDevice(c *gin.Context) {
	id := c.Param("id")
	var dev model.Device
	if err := h.DB.Where("id = ?", id).First(&dev).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}
	var req struct {
		Alias            *string           `json:"alias"`
		GroupID          *uint             `json:"group_id"`
		Note             *string           `json:"note"`
		ClientRole       *string           `json:"client_role"`
		StrategyOverride map[string]string `json:"strategy_override"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式错误"})
		return
	}
	updates := map[string]interface{}{}
	if req.Alias != nil {
		updates["alias"] = *req.Alias
	}
	if req.GroupID != nil {
		updates["group_id"] = *req.GroupID
	}
	if req.Note != nil {
		updates["note"] = *req.Note
	}
	if req.ClientRole != nil {
		if r := normalizeRole(*req.ClientRole); r != "" {
			updates["client_role"] = r
		}
	}
	if req.StrategyOverride != nil {
		b, _ := json.Marshal(req.StrategyOverride)
		updates["strategy_override"] = string(b)
		h.bumpGlobalStrategyTimestamp()
	}
	if len(updates) > 0 {
		h.DB.Model(&dev).Updates(updates)
	}
	h.DB.Where("id = ?", id).First(&dev)
	dev.Online = h.isDeviceOnline(dev.LastSeenAt)
	c.JSON(http.StatusOK, dev)
}

func (h *Handler) HandleBatchGroupDevices(c *gin.Context) {
	var req struct {
		DeviceIDs []string `json:"device_ids" binding:"required"`
		GroupID   uint     `json:"group_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	h.DB.Model(&model.Device{}).Where("id IN ?", req.DeviceIDs).Update("group_id", req.GroupID)
	h.bumpGlobalStrategyTimestamp()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) HandleDeleteDevice(c *gin.Context) {
	id := c.Param("id")
	h.DB.Where("id = ?", id).Delete(&model.Device{})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) HandleDisconnectDevice(c *gin.Context) {
	id := c.Param("id")
	var dev model.Device
	if err := h.DB.Where("id = ?", id).First(&dev).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}
	var conns []int
	if dev.ActiveConnsJSON != "" {
		_ = json.Unmarshal([]byte(dev.ActiveConnsJSON), &conns)
	}
	if len(conns) == 0 {
		c.JSON(http.StatusOK, gin.H{"ok": true, "queued": 0})
		return
	}
	b, _ := json.Marshal(conns)
	h.DB.Model(&dev).Update("pending_disconnect", string(b))
	c.JSON(http.StatusOK, gin.H{"ok": true, "queued": len(conns)})
}

func (h *Handler) HandleLiveSessions(c *gin.Context) {
	scope := strings.TrimSpace(c.DefaultQuery("scope", "all"))
	keyword := strings.TrimSpace(c.Query("keyword"))

	var groups []model.DeviceGroup
	h.DB.Find(&groups)
	groupNames := make(map[uint]string)
	groupPasses := make(map[uint]string)
	for _, g := range groups {
		groupNames[g.ID] = g.Name
		groupPasses[g.ID] = g.UnattendedPass
	}

	var allDevs []model.Device
	h.DB.Find(&allDevs)
	devMap := make(map[string]model.Device, len(allDevs))
	for _, d := range allDevs {
		devMap[d.ID] = d
	}

	query := h.DB.Model(&model.SessionRecord{})
	if scope == "active" {
		query = query.Where("status = ?", "active")
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where(
			"agent_id LIKE ? OR agent_name LIKE ? OR agent_local_ip LIKE ? OR agent_remote_ip LIKE ? OR console_id LIKE ? OR console_name LIKE ? OR console_local_ip LIKE ? OR console_remote_ip LIKE ? OR console_user LIKE ?",
			like, like, like, like, like, like, like, like, like,
		)
	}

	var rows []model.SessionRecord
	query.Order("CASE WHEN status = 'active' THEN 0 ELSE 1 END ASC, started_at DESC").Limit(200).Find(&rows)

	items := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		agentDev, hasAgent := devMap[r.AgentID]
		consoleDev, hasConsole := devMap[r.ConsoleID]

		agentName := r.AgentName
		agentLocalIP := r.AgentLocalIP
		agentRemoteIP := r.AgentRemoteIP
		agentOS := r.AgentOS
		agentPass := r.AgentPassword
		agentGroup := ""
		agentOnline := false
		if hasAgent {
			if agentDev.Alias != "" {
				agentName = agentDev.Alias
			} else if agentDev.Hostname != "" {
				agentName = agentDev.Hostname
			}
			if agentDev.LocalIPs != "" {
				agentLocalIP = agentDev.LocalIPs
			}
			if agentDev.RemoteIP != "" {
				agentRemoteIP = agentDev.RemoteIP
			}
			if agentDev.OS != "" {
				agentOS = agentDev.OS
			}
			if agentDev.TempPassword != "" {
				agentPass = agentDev.TempPassword
			} else if groupPasses[agentDev.GroupID] != "" {
				agentPass = groupPasses[agentDev.GroupID]
			}
			agentGroup = groupNames[agentDev.GroupID]
			agentOnline = h.isDeviceOnline(agentDev.LastSeenAt)
		}

		consoleName := r.ConsoleName
		consoleLocalIP := r.ConsoleLocalIP
		consoleRemoteIP := r.ConsoleRemoteIP
		consoleUser := r.ConsoleUser
		consoleOS := r.ConsoleOS
		if hasConsole {
			if consoleDev.Alias != "" {
				consoleName = consoleDev.Alias
			} else if consoleDev.Hostname != "" {
				consoleName = consoleDev.Hostname
			}
			if consoleDev.LocalIPs != "" {
				consoleLocalIP = consoleDev.LocalIPs
			}
			if consoleDev.RemoteIP != "" {
				consoleRemoteIP = consoleDev.RemoteIP
			}
			if consoleDev.OS != "" {
				consoleOS = consoleDev.OS
			}
			if consoleUser == "" {
				if consoleDev.BoundUser != "" {
					consoleUser = consoleDev.BoundUser
				} else {
					consoleUser = consoleDev.Username
				}
			}
		}

		items = append(items, gin.H{
			"id":                r.ID,
			"session_key":       r.SessionKey,
			"status":            r.Status,
			"conn_type":         r.ConnType,
			"source":            r.Source,
			"started_at":        r.StartedAt,
			"ended_at":          r.EndedAt,
			"last_active_at":    r.LastActiveAt,
			"agent_id":          r.AgentID,
			"agent_name":        agentName,
			"agent_local_ip":    agentLocalIP,
			"agent_remote_ip":   agentRemoteIP,
			"agent_os":          agentOS,
			"agent_password":    agentPass,
			"agent_group":       agentGroup,
			"agent_online":      agentOnline,
			"console_id":        r.ConsoleID,
			"console_name":      consoleName,
			"console_local_ip":  consoleLocalIP,
			"console_remote_ip": consoleRemoteIP,
			"console_user":      consoleUser,
			"console_os":        consoleOS,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total": len(items),
		"items": items,
	})
}

func (h *Handler) HandleListGroups(c *gin.Context) {
	var groups []model.DeviceGroup
	h.DB.Order("id ASC").Find(&groups)

	type countRow struct {
		GroupID uint
		Total   int64
	}
	var counts []countRow
	h.DB.Model(&model.Device{}).Select("group_id, count(*) as total").Group("group_id").Scan(&counts)
	countMap := make(map[uint]int64)
	for _, row := range counts {
		countMap[row.GroupID] = row.Total
	}

	items := make([]gin.H, 0, len(groups))
	for _, g := range groups {
		var strategy map[string]string
		_ = json.Unmarshal([]byte(g.StrategyJSON), &strategy)
		if strategy == nil {
			strategy = map[string]string{}
		}
		items = append(items, gin.H{
			"id":                  g.ID,
			"name":                g.Name,
			"description":         g.Description,
			"strategy":            strategy,
			"unattended_pass":     g.UnattendedPass,
			"strategy_updated_at": g.StrategyUpdatedAt,
			"device_count":        countMap[g.ID],
			"created_at":          g.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) HandleCreateGroup(c *gin.Context) {
	var req struct {
		Name           string            `json:"name" binding:"required"`
		Description    string            `json:"description"`
		Strategy       map[string]string `json:"strategy"`
		UnattendedPass string            `json:"unattended_pass"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分组名称不能为空"})
		return
	}
	if req.Strategy == nil {
		req.Strategy = map[string]string{}
	}
	sb, _ := json.Marshal(req.Strategy)
	grp := model.DeviceGroup{
		Name:              strings.TrimSpace(req.Name),
		Description:       req.Description,
		StrategyJSON:      string(sb),
		UnattendedPass:    req.UnattendedPass,
		StrategyUpdatedAt: time.Now().Unix(),
	}
	if err := h.DB.Create(&grp).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分组名称已存在或创建失败"})
		return
	}
	c.JSON(http.StatusOK, grp)
}

func (h *Handler) HandleUpdateGroup(c *gin.Context) {
	id := c.Param("id")
	var grp model.DeviceGroup
	if err := h.DB.First(&grp, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分组不存在"})
		return
	}
	var req struct {
		Name           *string           `json:"name"`
		Description    *string           `json:"description"`
		Strategy       map[string]string `json:"strategy"`
		UnattendedPass *string           `json:"unattended_pass"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		grp.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		grp.Description = *req.Description
	}
	if req.UnattendedPass != nil {
		grp.UnattendedPass = *req.UnattendedPass
	}
	if req.Strategy != nil {
		sb, _ := json.Marshal(req.Strategy)
		grp.StrategyJSON = string(sb)
		grp.StrategyUpdatedAt = time.Now().Unix()
	}
	if err := h.DB.Save(&grp).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "更新分组失败"})
		return
	}
	c.JSON(http.StatusOK, grp)
}

func (h *Handler) HandleDeleteGroup(c *gin.Context) {
	id := c.Param("id")
	if id == "1" || id == "2" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "系统内置默认分组不可删除"})
		return
	}
	h.DB.Model(&model.Device{}).Where("group_id = ?", id).Update("group_id", 1)
	h.DB.Delete(&model.DeviceGroup{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) HandleListUsers(c *gin.Context) {
	var users []model.User
	h.DB.Order("id ASC").Find(&users)
	items := make([]gin.H, 0, len(users))
	for _, u := range users {
		items = append(items, gin.H{
			"id":             u.ID,
			"username":       u.Username,
			"display_name":   u.DisplayName,
			"email":          u.Email,
			"role":           u.Role,
			"is_admin":       u.IsAdmin,
			"status":         u.Status,
			"allowed_groups": parseAllowedGroups(u.AllowedGroups),
			"note":           u.Note,
			"created_at":     u.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) HandleCreateUser(c *gin.Context) {
	var req struct {
		Username      string `json:"username" binding:"required"`
		DisplayName   string `json:"display_name"`
		Email         string `json:"email"`
		Password      string `json:"password" binding:"required"`
		Role          string `json:"role"`
		AllowedGroups []uint `json:"allowed_groups"`
		Note          string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写用户名及至少6位密码"})
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码处理失败"})
		return
	}
	role := req.Role
	if role != "admin" && role != "teacher" && role != "viewer" {
		role = "teacher"
	}
	ag, _ := json.Marshal(req.AllowedGroups)
	user := model.User{
		Username:      strings.TrimSpace(req.Username),
		DisplayName:   req.DisplayName,
		Email:         req.Email,
		PasswordHash:  hash,
		Role:          role,
		IsAdmin:       role == "admin",
		Status:        1,
		AllowedGroups: string(ag),
		Note:          req.Note,
	}
	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名已存在或创建失败"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *Handler) HandleUpdateUser(c *gin.Context) {
	id := c.Param("id")
	var user model.User
	if err := h.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账号不存在"})
		return
	}
	var req struct {
		DisplayName   *string `json:"display_name"`
		Email         *string `json:"email"`
		Password      *string `json:"password"`
		Role          *string `json:"role"`
		Status        *int    `json:"status"`
		AllowedGroups []uint  `json:"allowed_groups"`
		Note          *string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.DisplayName != nil {
		user.DisplayName = *req.DisplayName
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Password != nil && *req.Password != "" {
		if len(*req.Password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "密码至少6位"})
			return
		}
		if hash, err := auth.HashPassword(*req.Password); err == nil {
			user.PasswordHash = hash
		}
	}
	if req.Role != nil && (*req.Role == "admin" || *req.Role == "teacher" || *req.Role == "viewer") {
		user.Role = *req.Role
		user.IsAdmin = (*req.Role == "admin")
	}
	if req.Status != nil {
		user.Status = *req.Status
	}
	if req.AllowedGroups != nil {
		ag, _ := json.Marshal(req.AllowedGroups)
		user.AllowedGroups = string(ag)
	}
	if req.Note != nil {
		user.Note = *req.Note
	}
	h.DB.Save(&user)
	c.JSON(http.StatusOK, user)
}

func (h *Handler) HandleDeleteUser(c *gin.Context) {
	id := c.Param("id")
	current := c.MustGet("user").(*model.User)
	if strconv.FormatUint(uint64(current.ID), 10) == id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除当前登录账号"})
		return
	}
	h.DB.Delete(&model.User{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) HandleListAudits(c *gin.Context) {
	auditType := strings.TrimSpace(c.Query("audit_type"))
	deviceID := strings.TrimSpace(c.Query("device_id"))
	keyword := strings.TrimSpace(c.Query("keyword"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	query := h.DB.Model(&model.AuditLog{})
	if auditType != "" && auditType != "all" {
		query = query.Where("audit_type = ?", auditType)
	}
	if deviceID != "" {
		query = query.Where("device_id = ? OR peer_id = ?", deviceID, deviceID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("device_id LIKE ? OR peer_id LIKE ? OR peer_name LIKE ? OR operator_user LIKE ? OR client_ip LIKE ? OR note LIKE ?",
			like, like, like, like, like, like)
	}

	var total int64
	query.Count(&total)

	var items []model.AuditLog
	query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items)

	c.JSON(http.StatusOK, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     items,
	})
}

func (h *Handler) HandleGetSettings(c *gin.Context) {
	var globalStrategy map[string]string
	_ = json.Unmarshal([]byte(h.getSetting("global_strategy_json", "{}")), &globalStrategy)
	if globalStrategy == nil {
		globalStrategy = map[string]string{}
	}
	c.JSON(http.StatusOK, gin.H{
		"school_name":           h.getSetting("school_name", "校园远控运维平台"),
		"heartbeat_timeout_sec": h.getSetting("heartbeat_timeout_sec", "45"),
		"global_strategy":       globalStrategy,
		"strategy_updated_at":   h.getSetting("strategy_updated_at", "1"),
	})
}

func (h *Handler) HandleUpdateSettings(c *gin.Context) {
	var req struct {
		SchoolName          *string           `json:"school_name"`
		HeartbeatTimeoutSec *string           `json:"heartbeat_timeout_sec"`
		GlobalStrategy      map[string]string `json:"global_strategy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.SchoolName != nil {
		h.DB.Save(&model.Setting{Key: "school_name", Value: *req.SchoolName})
	}
	if req.HeartbeatTimeoutSec != nil {
		if sec, err := strconv.Atoi(*req.HeartbeatTimeoutSec); err == nil && sec >= 15 {
			h.DB.Save(&model.Setting{Key: "heartbeat_timeout_sec", Value: strconv.Itoa(sec)})
		}
	}
	if req.GlobalStrategy != nil {
		b, _ := json.Marshal(req.GlobalStrategy)
		h.DB.Save(&model.Setting{Key: "global_strategy_json", Value: string(b)})
		h.bumpGlobalStrategyTimestamp()
	}
	h.HandleGetSettings(c)
}
