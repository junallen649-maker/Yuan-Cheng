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

// --- Schools CRUD ---

func (h *Handler) HandleListSchools(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	query := h.DB.Order("id ASC")
	if current.SchoolID > 0 {
		query = query.Where("id = ?", current.SchoolID)
	}
	var schools []model.School
	query.Find(&schools)

	type countRow struct {
		SchoolID uint
		Total    int64
	}
	var devCounts []countRow
	h.DB.Model(&model.Device{}).Select("school_id, count(*) as total").Group("school_id").Scan(&devCounts)
	devCountMap := make(map[uint]int64)
	for _, r := range devCounts {
		devCountMap[r.SchoolID] = r.Total
	}

	var allDevs []model.Device
	h.DB.Select("id, school_id, last_seen_at").Find(&allDevs)
	onlineMap := make(map[uint]int)
	for _, d := range allDevs {
		if h.isDeviceOnline(d.LastSeenAt) {
			onlineMap[d.SchoolID]++
		}
	}

	var grpCounts []countRow
	h.DB.Model(&model.DeviceGroup{}).Select("school_id, count(*) as total").Group("school_id").Scan(&grpCounts)
	grpCountMap := make(map[uint]int64)
	for _, r := range grpCounts {
		grpCountMap[r.SchoolID] = r.Total
	}

	items := make([]gin.H, 0, len(schools))
	for _, s := range schools {
		var strat map[string]string
		_ = json.Unmarshal([]byte(s.DefaultStrategyJSON), &strat)
		if strat == nil {
			strat = map[string]string{}
		}
		items = append(items, gin.H{
			"id":               s.ID,
			"name":             s.Name,
			"code":             s.Code,
			"enroll_token":     s.EnrollToken,
			"id_seq":           s.IDSeq,
			"auto_assign_id":   s.AutoAssignID,
			"default_strategy": strat,
			"status":           s.Status,
			"contact":          s.Contact,
			"note":             s.Note,
			"device_count":     devCountMap[s.ID],
			"online_count":     onlineMap[s.ID],
			"group_count":      grpCountMap[s.ID],
			"created_at":       s.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) HandleCreateSchool(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	if !isSuperAdmin(current) {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅平台超级管理员可新增学校"})
		return
	}
	var req struct {
		Name            string            `json:"name" binding:"required"`
		Code            string            `json:"code" binding:"required"`
		EnrollToken     string            `json:"enroll_token"`
		AutoAssignID    bool              `json:"auto_assign_id"`
		DefaultStrategy map[string]string `json:"default_strategy"`
		Contact         string            `json:"contact"`
		Note            string            `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写学校名称和编码"})
		return
	}
	code := strings.ToLower(strings.TrimSpace(req.Code))
	if !IsValidSchoolCode(code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "学校编码需以字母开头，2~8位字母或数字（如 yz、sc01）"})
		return
	}
	enrollToken := strings.TrimSpace(req.EnrollToken)
	if enrollToken == "" {
		enrollToken = code + "-enroll"
	}
	if req.DefaultStrategy == nil {
		req.DefaultStrategy = map[string]string{
			"approve-mode":        "password",
			"verification-method": "use-both-passwords",
		}
	}
	sb, _ := json.Marshal(req.DefaultStrategy)
	school := model.School{
		Name:                strings.TrimSpace(req.Name),
		Code:                code,
		EnrollToken:         enrollToken,
		AutoAssignID:        req.AutoAssignID,
		DefaultStrategyJSON: string(sb),
		Status:              1,
		Contact:             req.Contact,
		Note:                req.Note,
	}
	if err := h.DB.Create(&school).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "学校名称、编码或入网码已存在"})
		return
	}
	h.ensureSchoolDefaultGroup(h.DB, school.ID)
	c.JSON(http.StatusOK, school)
}

func (h *Handler) HandleUpdateSchool(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	id := c.Param("id")
	var school model.School
	if err := h.DB.First(&school, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "学校不存在"})
		return
	}
	if !isSuperAdmin(current) && current.SchoolID != school.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权修改其他学校"})
		return
	}
	var req struct {
		Name            *string           `json:"name"`
		Code            *string           `json:"code"`
		EnrollToken     *string           `json:"enroll_token"`
		AutoAssignID    *bool             `json:"auto_assign_id"`
		DefaultStrategy map[string]string `json:"default_strategy"`
		Status          *int              `json:"status"`
		Contact         *string           `json:"contact"`
		Note            *string           `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		school.Name = strings.TrimSpace(*req.Name)
	}
	if req.Code != nil && strings.TrimSpace(*req.Code) != "" {
		code := strings.ToLower(strings.TrimSpace(*req.Code))
		if !IsValidSchoolCode(code) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "学校编码需以字母开头，2~8位字母或数字"})
			return
		}
		school.Code = code
	}
	if req.EnrollToken != nil && strings.TrimSpace(*req.EnrollToken) != "" {
		school.EnrollToken = strings.TrimSpace(*req.EnrollToken)
	}
	if req.AutoAssignID != nil {
		school.AutoAssignID = *req.AutoAssignID
	}
	if req.Status != nil {
		school.Status = *req.Status
	}
	if req.Contact != nil {
		school.Contact = *req.Contact
	}
	if req.Note != nil {
		school.Note = *req.Note
	}
	if req.DefaultStrategy != nil {
		sb, _ := json.Marshal(req.DefaultStrategy)
		school.DefaultStrategyJSON = string(sb)
	}
	if err := h.DB.Save(&school).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "更新学校失败（名称或编码可能重复）"})
		return
	}
	c.JSON(http.StatusOK, school)
}

func (h *Handler) HandleDeleteSchool(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	if !isSuperAdmin(current) {
		c.JSON(http.StatusForbidden, gin.H{"error": "仅平台超级管理员可删除学校"})
		return
	}
	id := c.Param("id")
	if id == "1" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "系统内置默认学校不可删除"})
		return
	}
	h.DB.Model(&model.Device{}).Where("school_id = ?", id).Updates(map[string]interface{}{"school_id": 1, "group_id": 1})
	h.DB.Model(&model.DeviceGroup{}).Where("school_id = ?", id).Delete(&model.DeviceGroup{})
	h.DB.Delete(&model.School{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- Overview ---

func (h *Handler) HandleOverview(c *gin.Context) {
	current := c.MustGet("user").(*model.User)

	devQuery := h.DB.Model(&model.Device{})
	grpQuery := h.DB.Model(&model.DeviceGroup{})
	if current.SchoolID > 0 {
		devQuery = devQuery.Where("school_id = ?", current.SchoolID)
		grpQuery = grpQuery.Where("school_id = ?", current.SchoolID)
	}

	var devices []model.Device
	devQuery.Find(&devices)

	var groups []model.DeviceGroup
	grpQuery.Find(&groups)
	groupNames := make(map[uint]string)
	for _, g := range groups {
		groupNames[g.ID] = g.Name
	}

	var schools []model.School
	h.DB.Find(&schools)
	schoolNames := make(map[uint]string)
	for _, s := range schools {
		schoolNames[s.ID] = s.Name
	}

	var totalAgents, onlineAgents, totalConsoles, onlineConsoles, activeConns, assignedCount int
	groupDist := make(map[string]map[string]int)

	for _, d := range devices {
		online := h.isDeviceOnline(d.LastSeenAt)
		gName := groupNames[d.GroupID]
		if gName == "" {
			gName = "未分组"
		}
		if current.SchoolID == 0 && schoolNames[d.SchoolID] != "" {
			gName = schoolNames[d.SchoolID] + " · " + gName
		}
		if _, ok := groupDist[gName]; !ok {
			groupDist[gName] = map[string]int{"total": 0, "online": 0}
		}
		groupDist[gName]["total"]++
		if online {
			groupDist[gName]["online"]++
		}
		if d.IDAssigned {
			assignedCount++
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
	sessQ := h.DB.Model(&model.SessionRecord{}).Where("status = ?", "active")
	if current.SchoolID > 0 {
		var devIDs []string
		for _, d := range devices {
			devIDs = append(devIDs, d.ID)
		}
		if len(devIDs) > 0 {
			sessQ = sessQ.Where("agent_id IN ? OR console_id IN ?", devIDs, devIDs)
		} else {
			sessQ = sessQ.Where("1 = 0")
		}
	}
	sessQ.Count(&activeSessionRows)
	if int(activeSessionRows) > activeConns {
		activeConns = int(activeSessionRows)
	}

	todayStart := time.Now().Truncate(24 * time.Hour)
	var todayAudits int64
	h.DB.Model(&model.AuditLog{}).Where("created_at >= ?", todayStart).Count(&todayAudits)

	var totalUsers, totalSchools int64
	uQ := h.DB.Model(&model.User{}).Where("status = 1")
	if current.SchoolID > 0 {
		uQ = uQ.Where("school_id = ?", current.SchoolID)
	}
	uQ.Count(&totalUsers)
	h.DB.Model(&model.School{}).Where("status = 1").Count(&totalSchools)

	var recentAudits []model.AuditLog
	h.DB.Order("created_at DESC").Limit(8).Find(&recentAudits)

	c.JSON(http.StatusOK, gin.H{
		"school_name":      h.getSetting("school_name", "Polaris 多校远控运维平台"),
		"server_host":      h.Cfg.ServerPublicHost,
		"rendezvous_port":  h.Cfg.RendezvousPort,
		"relay_port":       h.Cfg.RelayPort,
		"total_schools":    totalSchools,
		"total_devices":    len(devices),
		"assigned_devices": assignedCount,
		"total_agents":     totalAgents,
		"online_agents":    onlineAgents,
		"total_consoles":   totalConsoles,
		"online_consoles":  onlineConsoles,
		"active_sessions":  activeConns,
		"today_audits":     todayAudits,
		"total_users":      totalUsers,
		"group_stats":      groupDist,
		"recent_audits":    recentAudits,
	})
}

// --- Devices ---

func (h *Handler) HandleListDevices(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	role := strings.TrimSpace(c.Query("role"))
	status := strings.TrimSpace(c.Query("status"))
	schoolIDStr := strings.TrimSpace(c.Query("school_id"))
	groupIDStr := strings.TrimSpace(c.Query("group_id"))
	keyword := strings.TrimSpace(c.Query("keyword"))

	var schools []model.School
	h.DB.Find(&schools)
	schoolNames := make(map[uint]string)
	for _, s := range schools {
		schoolNames[s.ID] = s.Name
	}

	var groups []model.DeviceGroup
	h.DB.Find(&groups)
	groupNames := make(map[uint]string)
	groupPasses := make(map[uint]string)
	for _, g := range groups {
		groupNames[g.ID] = g.Name
		groupPasses[g.ID] = g.UnattendedPass
	}

	query := h.DB.Model(&model.Device{}).Order("last_seen_at DESC")
	if current.SchoolID > 0 {
		query = query.Where("school_id = ?", current.SchoolID)
	} else if schoolIDStr != "" && schoolIDStr != "0" {
		if sid, err := strconv.Atoi(schoolIDStr); err == nil {
			query = query.Where("school_id = ?", sid)
		}
	}
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

	// Attach latest command status per device
	var recentCmds []model.DeviceCommand
	h.DB.Order("id DESC").Limit(500).Find(&recentCmds)
	latestCmdMap := make(map[string]string)
	for _, cmd := range recentCmds {
		if _, ok := latestCmdMap[cmd.DeviceID]; !ok {
			latestCmdMap[cmd.DeviceID] = cmd.CommandType + ":" + cmd.Status
			if cmd.ResultMsg != "" {
				latestCmdMap[cmd.DeviceID] += " (" + cmd.ResultMsg + ")"
			}
		}
	}

	filtered := make([]model.Device, 0, len(devices))
	for _, d := range devices {
		d.Online = h.isDeviceOnline(d.LastSeenAt)
		d.SchoolName = schoolNames[d.SchoolID]
		d.GroupName = groupNames[d.GroupID]
		d.UnattendedPass = groupPasses[d.GroupID]
		d.LatestCmdInfo = latestCmdMap[d.ID]
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
	current := c.MustGet("user").(*model.User)
	id := c.Param("id")
	var dev model.Device
	if err := h.DB.Where("id = ?", id).First(&dev).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}
	if current.SchoolID > 0 && dev.SchoolID != current.SchoolID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权操作其他学校设备"})
		return
	}
	var req struct {
		Alias            *string           `json:"alias"`
		SchoolID         *uint             `json:"school_id"`
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
	if req.SchoolID != nil && isSuperAdmin(current) && *req.SchoolID > 0 {
		updates["school_id"] = *req.SchoolID
		if req.GroupID == nil {
			updates["group_id"] = h.ensureSchoolDefaultGroup(h.DB, *req.SchoolID)
		}
	}
	if req.GroupID != nil {
		var grp model.DeviceGroup
		if h.DB.First(&grp, *req.GroupID).Error == nil {
			if current.SchoolID > 0 && grp.SchoolID != current.SchoolID {
				c.JSON(http.StatusForbidden, gin.H{"error": "不能跨学校分配分组"})
				return
			}
			updates["group_id"] = grp.ID
			updates["school_id"] = grp.SchoolID
		}
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
	current := c.MustGet("user").(*model.User)
	var req struct {
		DeviceIDs []string `json:"device_ids" binding:"required"`
		GroupID   uint     `json:"group_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	var grp model.DeviceGroup
	if err := h.DB.First(&grp, req.GroupID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "目标分组不存在"})
		return
	}
	if current.SchoolID > 0 && grp.SchoolID != current.SchoolID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权跨学校调拨设备"})
		return
	}
	q := h.DB.Model(&model.Device{}).Where("id IN ?", req.DeviceIDs)
	if current.SchoolID > 0 {
		q = q.Where("school_id = ?", current.SchoolID)
	}
	q.Updates(map[string]interface{}{"group_id": grp.ID, "school_id": grp.SchoolID})
	h.bumpGlobalStrategyTimestamp()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) HandleDeleteDevice(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	id := c.Param("id")
	q := h.DB.Where("id = ?", id)
	if current.SchoolID > 0 {
		q = q.Where("school_id = ?", current.SchoolID)
	}
	q.Delete(&model.Device{})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) HandleDisconnectDevice(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	id := c.Param("id")
	var dev model.Device
	if err := h.DB.Where("id = ?", id).First(&dev).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}
	if current.SchoolID > 0 && dev.SchoolID != current.SchoolID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权操作其他学校设备"})
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

// --- Remote ID / Password Commands ---

func (h *Handler) HandleSendDeviceCommand(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	id := c.Param("id")
	var dev model.Device
	if err := h.DB.Where("id = ?", id).First(&dev).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "设备不存在"})
		return
	}
	if current.SchoolID > 0 && dev.SchoolID != current.SchoolID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权操作其他学校设备"})
		return
	}

	var req struct {
		Type     string `json:"type" binding:"required"` // change_id | set_temp_password | refresh_temp_password | set_permanent_password
		NewID    string `json:"new_id"`
		Password string `json:"password"`
		AutoGen  bool   `json:"auto_gen"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	var createdCmd *model.DeviceCommand
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		switch req.Type {
		case "change_id":
			newID := strings.TrimSpace(req.NewID)
			if req.AutoGen || newID == "" {
				var sch model.School
				if err := tx.First(&sch, dev.SchoolID).Error; err != nil {
					return err
				}
				gen, err := h.nextSchoolIDTx(tx, &sch)
				if err != nil {
					return err
				}
				newID = gen
			} else {
				if !IsValidCustomID(newID) {
					return gorm.ErrInvalidData
				}
				var count int64
				tx.Model(&model.Device{}).Where("id = ? AND id != ?", newID, dev.ID).Count(&count)
				if count > 0 {
					return gorm.ErrDuplicatedKey
				}
			}
			cmd, err := h.queueDeviceCommand(tx, dev.ID, "change_id", map[string]string{"new_id": newID}, current.Username)
			createdCmd = cmd
			return err
		case "set_temp_password":
			pw := strings.TrimSpace(req.Password)
			if len(pw) < 4 {
				return gorm.ErrInvalidData
			}
			cmd, err := h.queueDeviceCommand(tx, dev.ID, "set_temp_password", map[string]string{"password": pw}, current.Username)
			createdCmd = cmd
			return err
		case "refresh_temp_password":
			cmd, err := h.queueDeviceCommand(tx, dev.ID, "refresh_temp_password", map[string]string{}, current.Username)
			createdCmd = cmd
			return err
		case "set_permanent_password":
			pw := strings.TrimSpace(req.Password)
			if pw != "" && len(pw) < 6 {
				return gorm.ErrInvalidData
			}
			cmd, err := h.queueDeviceCommand(tx, dev.ID, "set_permanent_password", map[string]string{"password": pw}, current.Username)
			createdCmd = cmd
			return err
		default:
			return gorm.ErrInvalidData
		}
	})

	if err == gorm.ErrInvalidData {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式不符合要求（连接代码需字母开头6~16位，临时密码>=4位，固定密码>=6位或留空清除）"})
		return
	}
	if err == gorm.ErrDuplicatedKey {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该连接代码已被其他设备占用"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, createdCmd)
}

func (h *Handler) HandleAssignIDs(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	var req struct {
		SchoolID  uint     `json:"school_id"`
		DeviceIDs []string `json:"device_ids"`
		OnlyUnassigned bool `json:"only_unassigned"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}

	targetSchoolID := req.SchoolID
	if current.SchoolID > 0 {
		targetSchoolID = current.SchoolID
	}

	var queued int
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&model.Device{}).Where("client_role != ?", "console")
		if len(req.DeviceIDs) > 0 {
			q = q.Where("id IN ?", req.DeviceIDs)
		}
		if targetSchoolID > 0 {
			q = q.Where("school_id = ?", targetSchoolID)
		}
		if req.OnlyUnassigned {
			q = q.Where("id_assigned = ?", false)
		}
		var devs []model.Device
		q.Find(&devs)

		schoolCache := make(map[uint]*model.School)
		for _, d := range devs {
			sid := d.SchoolID
			if targetSchoolID > 0 && isSuperAdmin(current) {
				sid = targetSchoolID
				if d.SchoolID != targetSchoolID {
					grpID := h.ensureSchoolDefaultGroup(tx, targetSchoolID)
					tx.Model(&model.Device{}).Where("id = ?", d.ID).Updates(map[string]interface{}{
						"school_id": targetSchoolID,
						"group_id":  grpID,
					})
				}
			}
			sch, ok := schoolCache[sid]
			if !ok {
				var s model.School
				if err := tx.First(&s, sid).Error; err != nil {
					continue
				}
				sch = &s
				schoolCache[sid] = sch
			}
			newID, err := h.nextSchoolIDTx(tx, sch)
			if err != nil {
				return err
			}
			if _, err := h.queueDeviceCommand(tx, d.ID, "change_id", map[string]string{"new_id": newID}, current.Username); err != nil {
				return err
			}
			queued++
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "queued": queued})
}

func (h *Handler) HandleBatchPassword(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	var req struct {
		Type      string   `json:"type" binding:"required"` // set_permanent_password | set_temp_password | refresh_temp_password
		Password  string   `json:"password"`
		SchoolID  uint     `json:"school_id"`
		GroupID   uint     `json:"group_id"`
		DeviceIDs []string `json:"device_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	pw := strings.TrimSpace(req.Password)
	if req.Type == "set_permanent_password" && pw != "" && len(pw) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "固定密码不能少于6位（或留空清除）"})
		return
	}
	if req.Type == "set_temp_password" && len(pw) < 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "临时密码不能少于4位"})
		return
	}

	q := h.DB.Model(&model.Device{}).Where("client_role != ?", "console")
	if current.SchoolID > 0 {
		q = q.Where("school_id = ?", current.SchoolID)
	} else if req.SchoolID > 0 {
		q = q.Where("school_id = ?", req.SchoolID)
	}
	if req.GroupID > 0 {
		q = q.Where("group_id = ?", req.GroupID)
	}
	if len(req.DeviceIDs) > 0 {
		q = q.Where("id IN ?", req.DeviceIDs)
	}
	var devs []model.Device
	q.Find(&devs)

	payload := map[string]string{}
	if req.Type != "refresh_temp_password" {
		payload["password"] = pw
	}

	queued := 0
	_ = h.DB.Transaction(func(tx *gorm.DB) error {
		for _, d := range devs {
			if _, err := h.queueDeviceCommand(tx, d.ID, req.Type, payload, current.Username); err == nil {
				queued++
			}
		}
		if req.Type == "set_permanent_password" && req.GroupID > 0 {
			tx.Model(&model.DeviceGroup{}).Where("id = ?", req.GroupID).Update("unattended_pass", pw)
		}
		return nil
	})
	c.JSON(http.StatusOK, gin.H{"ok": true, "queued": queued})
}

func (h *Handler) HandleListCommands(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	deviceID := strings.TrimSpace(c.Query("device_id"))

	q := h.DB.Model(&model.DeviceCommand{})
	if current.SchoolID > 0 {
		var devIDs []string
		h.DB.Model(&model.Device{}).Where("school_id = ?", current.SchoolID).Pluck("id", &devIDs)
		if len(devIDs) == 0 {
			c.JSON(http.StatusOK, gin.H{"items": []interface{}{}})
			return
		}
		q = q.Where("device_id IN ?", devIDs)
	}
	if deviceID != "" {
		q = q.Where("device_id = ?", deviceID)
	}
	var items []model.DeviceCommand
	q.Order("id DESC").Limit(100).Find(&items)
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// --- Sessions ---

func (h *Handler) HandleLiveSessions(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	scope := strings.TrimSpace(c.DefaultQuery("scope", "all"))
	keyword := strings.TrimSpace(c.Query("keyword"))

	var schools []model.School
	h.DB.Find(&schools)
	schoolNames := make(map[uint]string)
	for _, s := range schools {
		schoolNames[s.ID] = s.Name
	}

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
	var schoolDevIDs []string
	for _, d := range allDevs {
		devMap[d.ID] = d
		if current.SchoolID > 0 && d.SchoolID == current.SchoolID {
			schoolDevIDs = append(schoolDevIDs, d.ID)
		}
	}

	query := h.DB.Model(&model.SessionRecord{})
	if current.SchoolID > 0 {
		if len(schoolDevIDs) == 0 {
			c.JSON(http.StatusOK, gin.H{"total": 0, "items": []interface{}{}})
			return
		}
		query = query.Where("agent_id IN ? OR console_id IN ?", schoolDevIDs, schoolDevIDs)
	}
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
		agentSchool := ""
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
			agentSchool = schoolNames[agentDev.SchoolID]
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
			"agent_school":      agentSchool,
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

// --- Groups ---

func (h *Handler) HandleListGroups(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	schoolIDStr := strings.TrimSpace(c.Query("school_id"))

	var schools []model.School
	h.DB.Find(&schools)
	schoolNames := make(map[uint]string)
	for _, s := range schools {
		schoolNames[s.ID] = s.Name
	}

	query := h.DB.Order("school_id ASC, id ASC")
	if current.SchoolID > 0 {
		query = query.Where("school_id = ?", current.SchoolID)
	} else if schoolIDStr != "" && schoolIDStr != "0" {
		if sid, err := strconv.Atoi(schoolIDStr); err == nil {
			query = query.Where("school_id = ?", sid)
		}
	}

	var groups []model.DeviceGroup
	query.Find(&groups)

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
			"school_id":           g.SchoolID,
			"school_name":         schoolNames[g.SchoolID],
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
	current := c.MustGet("user").(*model.User)
	var req struct {
		SchoolID       uint              `json:"school_id"`
		Name           string            `json:"name" binding:"required"`
		Description    string            `json:"description"`
		Strategy       map[string]string `json:"strategy"`
		UnattendedPass string            `json:"unattended_pass"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分组名称不能为空"})
		return
	}
	sid := req.SchoolID
	if current.SchoolID > 0 {
		sid = current.SchoolID
	}
	if sid == 0 {
		sid = 1
	}
	if req.Strategy == nil {
		req.Strategy = map[string]string{}
	}
	sb, _ := json.Marshal(req.Strategy)
	grp := model.DeviceGroup{
		SchoolID:          sid,
		Name:              strings.TrimSpace(req.Name),
		Description:       req.Description,
		StrategyJSON:      string(sb),
		UnattendedPass:    req.UnattendedPass,
		StrategyUpdatedAt: time.Now().Unix(),
	}
	if err := h.DB.Create(&grp).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "同校下分组名称已存在或创建失败"})
		return
	}
	c.JSON(http.StatusOK, grp)
}

func (h *Handler) HandleUpdateGroup(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	id := c.Param("id")
	var grp model.DeviceGroup
	if err := h.DB.First(&grp, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分组不存在"})
		return
	}
	if current.SchoolID > 0 && grp.SchoolID != current.SchoolID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权修改其他学校分组"})
		return
	}
	var req struct {
		SchoolID       *uint             `json:"school_id"`
		Name           *string           `json:"name"`
		Description    *string           `json:"description"`
		Strategy       map[string]string `json:"strategy"`
		UnattendedPass *string           `json:"unattended_pass"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.SchoolID != nil && isSuperAdmin(current) && *req.SchoolID > 0 {
		grp.SchoolID = *req.SchoolID
		h.DB.Model(&model.Device{}).Where("group_id = ?", grp.ID).Update("school_id", grp.SchoolID)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "更新分组失败（同校下名称可能重复）"})
		return
	}
	c.JSON(http.StatusOK, grp)
}

func (h *Handler) HandleDeleteGroup(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	id := c.Param("id")
	if id == "1" || id == "2" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "系统内置默认分组不可删除"})
		return
	}
	var grp model.DeviceGroup
	if err := h.DB.First(&grp, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "分组不存在"})
		return
	}
	if current.SchoolID > 0 && grp.SchoolID != current.SchoolID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权删除其他学校分组"})
		return
	}
	fallbackGroupID := h.ensureSchoolDefaultGroup(h.DB, grp.SchoolID)
	if fallbackGroupID == grp.ID {
		fallbackGroupID = 1
	}
	h.DB.Model(&model.Device{}).Where("group_id = ?", id).Update("group_id", fallbackGroupID)
	h.DB.Delete(&model.DeviceGroup{}, id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- Users ---

func (h *Handler) HandleListUsers(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	var schools []model.School
	h.DB.Find(&schools)
	schoolNames := make(map[uint]string)
	schoolNames[0] = "全平台"
	for _, s := range schools {
		schoolNames[s.ID] = s.Name
	}

	q := h.DB.Order("id ASC")
	if current.SchoolID > 0 {
		q = q.Where("school_id = ?", current.SchoolID)
	}
	var users []model.User
	q.Find(&users)

	items := make([]gin.H, 0, len(users))
	for _, u := range users {
		items = append(items, gin.H{
			"id":             u.ID,
			"username":       u.Username,
			"display_name":   u.DisplayName,
			"email":          u.Email,
			"role":           u.Role,
			"is_admin":       u.IsAdmin,
			"school_id":      u.SchoolID,
			"school_name":    schoolNames[u.SchoolID],
			"status":         u.Status,
			"allowed_groups": parseAllowedGroups(u.AllowedGroups),
			"note":           u.Note,
			"created_at":     u.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handler) HandleCreateUser(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
	var req struct {
		Username      string `json:"username" binding:"required"`
		DisplayName   string `json:"display_name"`
		Email         string `json:"email"`
		Password      string `json:"password" binding:"required"`
		Role          string `json:"role"`
		SchoolID      uint   `json:"school_id"`
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
	sid := req.SchoolID
	if current.SchoolID > 0 {
		sid = current.SchoolID
		if role == "admin" {
			role = "teacher"
		}
	}
	ag, _ := json.Marshal(req.AllowedGroups)
	user := model.User{
		Username:      strings.TrimSpace(req.Username),
		DisplayName:   req.DisplayName,
		Email:         req.Email,
		PasswordHash:  hash,
		Role:          role,
		IsAdmin:       role == "admin",
		SchoolID:      sid,
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
	current := c.MustGet("user").(*model.User)
	id := c.Param("id")
	var user model.User
	if err := h.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "账号不存在"})
		return
	}
	if current.SchoolID > 0 && user.SchoolID != current.SchoolID {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权操作其他学校账号"})
		return
	}
	var req struct {
		DisplayName   *string `json:"display_name"`
		Email         *string `json:"email"`
		Password      *string `json:"password"`
		Role          *string `json:"role"`
		SchoolID      *uint   `json:"school_id"`
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
		if current.SchoolID > 0 && *req.Role == "admin" {
			user.Role = "teacher"
			user.IsAdmin = false
		} else {
			user.Role = *req.Role
			user.IsAdmin = (*req.Role == "admin")
		}
	}
	if req.SchoolID != nil && isSuperAdmin(current) {
		user.SchoolID = *req.SchoolID
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
	q := h.DB.Where("id = ?", id)
	if current.SchoolID > 0 {
		q = q.Where("school_id = ?", current.SchoolID)
	}
	q.Delete(&model.User{})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- Audits & Settings ---

func (h *Handler) HandleListAudits(c *gin.Context) {
	current := c.MustGet("user").(*model.User)
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
	if current.SchoolID > 0 {
		var devIDs []string
		h.DB.Model(&model.Device{}).Where("school_id = ?", current.SchoolID).Pluck("id", &devIDs)
		if len(devIDs) == 0 {
			c.JSON(http.StatusOK, gin.H{"total": 0, "page": page, "page_size": pageSize, "items": []interface{}{}})
			return
		}
		query = query.Where("device_id IN ? OR peer_id IN ?", devIDs, devIDs)
	}
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
		"school_name":           h.getSetting("school_name", "Polaris 多校远控运维平台"),
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
