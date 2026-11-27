package handler

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"polaris-platform/internal/auth"
	"polaris-platform/internal/config"
	"polaris-platform/internal/model"

	"github.com/gin-gonic/gin"
)

func setupTestHandler(t *testing.T) (*Handler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, secret, err := model.InitDB(dbPath, "admin", "polaris2026", "test-secret")
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	cfg := config.Load()
	cfg.RendezvousPort = 0
	cfg.ServerPublicHost = ""
	h := New(db, cfg, secret)

	r := gin.New()
	r.POST("/api/sysinfo", h.HandleSysinfo)
	r.POST("/api/heartbeat", h.HandleHeartbeat)
	r.POST("/api/command/ack", h.HandleCommandAck)
	r.POST("/api/login", h.HandleClientLogin)

	authedClient := r.Group("/api")
	authedClient.Use(auth.RequireUser(db, secret))
	{
		authedClient.POST("/currentUser", h.HandleCurrentUser)
		authedClient.GET("/device-group/accessible", h.HandleAccessibleGroups)
		authedClient.GET("/users", h.HandleClientUsers)
		authedClient.GET("/peers", h.HandleClientPeers)
		authedClient.POST("/ab/personal", h.HandleABPersonal)
		authedClient.GET("/ab", h.HandleGetAB)
	}

	admin := r.Group("/api/admin")
	admin.Use(auth.RequireUser(db, secret))
	{
		admin.GET("/devices", h.HandleListDevices)
		admin.POST("/devices/probe", h.HandleProbeDevices)
		admin.POST("/users", h.HandleCreateUser)
		admin.PUT("/devices/:id", h.HandleUpdateDevice)
		admin.PUT("/groups/:id", h.HandleUpdateGroup)
		admin.DELETE("/groups/:id", h.HandleDeleteGroup)
	}
	return h, r
}

func TestMultiSchoolEnrollAutoAssignAndAckMigration(t *testing.T) {
	h, r := setupTestHandler(t)

	// Create School A (code: yz, auto_assign_id: true)
	schoolA := model.School{
		Name:         "实验一中",
		Code:         "yz",
		EnrollToken:  "yz-token",
		AutoAssignID: true,
		Status:       1,
	}
	if err := h.DB.Create(&schoolA).Error; err != nil {
		t.Fatalf("create schoolA: %v", err)
	}
	h.ensureSchoolDefaultGroup(h.DB, schoolA.ID)

	// Device registers with school_code = "yz" and temp numeric ID "123456789"
	body, _ := json.Marshal(map[string]interface{}{
		"id":            "123456789",
		"uuid":          "uuid-1",
		"version":       "1.5.0",
		"hostname":      "LAB-01",
		"client_role":   "agent",
		"school_code":   "yz",
		"temp_password": "initpw",
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/sysinfo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("sysinfo failed: %d %s", w.Code, w.Body.String())
	}

	// Heartbeat should return change_id command "yz-0001"
	hbBody, _ := json.Marshal(map[string]interface{}{
		"id":          "123456789",
		"uuid":        "uuid-1",
		"client_role": "agent",
	})
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/heartbeat", bytes.NewReader(hbBody))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("heartbeat failed: %d", w2.Code)
	}
	var hbResp struct {
		Commands []struct {
			ID      uint              `json:"id"`
			Type    string            `json:"type"`
			Payload map[string]string `json:"payload"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &hbResp); err != nil {
		t.Fatalf("unmarshal heartbeat: %v", err)
	}
	if len(hbResp.Commands) != 1 || hbResp.Commands[0].Type != "change_id" || hbResp.Commands[0].Payload["new_id"] != "yz-0001" {
		t.Fatalf("unexpected commands: %+v", hbResp.Commands)
	}
	if !IsValidCustomID(hbResp.Commands[0].Payload["new_id"]) {
		t.Fatalf("generated id invalid for Polaris: %s", hbResp.Commands[0].Payload["new_id"])
	}

	// Ack command success -> device row should migrate from 123456789 to yz-0001
	ackBody, _ := json.Marshal(map[string]interface{}{
		"cmd_id":        hbResp.Commands[0].ID,
		"device_id":     "123456789",
		"status":        "succeeded",
		"old_id":        "123456789",
		"new_id":        "yz-0001",
		"temp_password": "initpw",
	})
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodPost, "/api/command/ack", bytes.NewReader(ackBody))
	req3.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("ack failed: %d %s", w3.Code, w3.Body.String())
	}

	var oldCount, newCount int64
	h.DB.Model(&model.Device{}).Where("id = ?", "123456789").Count(&oldCount)
	h.DB.Model(&model.Device{}).Where("id = ?", "yz-0001").Count(&newCount)
	if oldCount != 0 || newCount != 1 {
		t.Fatalf("expected migration from 123456789 to yz-0001, got old=%d new=%d", oldCount, newCount)
	}
}

func TestClientLoginSchoolIsolationAndGroupDelete(t *testing.T) {
	h, r := setupTestHandler(t)

	// Create two schools (School 2 and School 3)
	school2 := model.School{Name: "第一实验中学", Code: "sy01", EnrollToken: "sy01-enroll", Status: 1}
	school3 := model.School{Name: "第二中学", Code: "ez02", EnrollToken: "ez02-enroll", Status: 1}
	if err := h.DB.Create(&school2).Error; err != nil {
		t.Fatalf("create school2: %v", err)
	}
	if err := h.DB.Create(&school3).Error; err != nil {
		t.Fatalf("create school3: %v", err)
	}
	grp2 := h.ensureSchoolDefaultGroup(h.DB, school2.ID)
	grp3 := h.ensureSchoolDefaultGroup(h.DB, school3.ID)

	// Seed devices in School 1, School 2, School 3
	devs := []model.Device{
		{ID: "dev-s1", ClientRole: "agent", SchoolID: 1, GroupID: 1, Hostname: "S1-PC"},
		{ID: "dev-s2-a", ClientRole: "agent", SchoolID: school2.ID, GroupID: grp2, Hostname: "S2-PC-A"},
		{ID: "dev-s2-b", ClientRole: "agent", SchoolID: school2.ID, GroupID: grp2, Hostname: "S2-PC-B"},
		{ID: "dev-s3", ClientRole: "agent", SchoolID: school3.ID, GroupID: grp3, Hostname: "S3-PC"},
	}
	for _, d := range devs {
		if err := h.DB.Create(&d).Error; err != nil {
			t.Fatalf("create device %s: %v", d.ID, err)
		}
	}

	// Super-admin creates customer account for School 2 via /api/admin/users
	var adminUser model.User
	if err := h.DB.Where("username = ?", "admin").First(&adminUser).Error; err != nil {
		t.Fatalf("find admin: %v", err)
	}
	adminToken, err := auth.GenerateToken(h.JWTSecret, &adminUser, 3600e9)
	if err != nil {
		t.Fatalf("admin token: %v", err)
	}

	createUserBody, _ := json.Marshal(map[string]interface{}{
		"username":     "sy_teacher",
		"display_name": "实验一中老师",
		"password":     "pass1234",
		"role":         "teacher",
		"school_id":    school2.ID,
	})
	wCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/admin/users", bytes.NewReader(createUserBody))
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate.Header.Set("Authorization", "Bearer "+adminToken)
	r.ServeHTTP(wCreate, reqCreate)
	if wCreate.Code != http.StatusOK {
		t.Fatalf("create user failed: %d %s", wCreate.Code, wCreate.Body.String())
	}

	// Customer logs in via client /api/login
	loginBody, _ := json.Marshal(map[string]interface{}{
		"username": "sy_teacher",
		"password": "pass1234",
	})
	wLogin := httptest.NewRecorder()
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wLogin, reqLogin)
	if wLogin.Code != http.StatusOK {
		t.Fatalf("client login failed: %d %s", wLogin.Code, wLogin.Body.String())
	}
	var loginResp struct {
		AccessToken string `json:"access_token"`
		User        struct {
			Name       string `json:"name"`
			SchoolID   uint   `json:"school_id"`
			SchoolName string `json:"school_name"`
		} `json:"user"`
	}
	if err := json.Unmarshal(wLogin.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("unmarshal login: %v", err)
	}
	if loginResp.AccessToken == "" || loginResp.User.SchoolID != school2.ID || loginResp.User.SchoolName != "第一实验中学" {
		t.Fatalf("unexpected login response: %+v", loginResp)
	}

	// Customer queries /api/peers -> must ONLY see School 2 devices
	wPeers := httptest.NewRecorder()
	reqPeers := httptest.NewRequest(http.MethodGet, "/api/peers", nil)
	reqPeers.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	r.ServeHTTP(wPeers, reqPeers)
	if wPeers.Code != http.StatusOK {
		t.Fatalf("peers failed: %d", wPeers.Code)
	}
	var peersResp struct {
		Total int `json:"total"`
		Data  []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(wPeers.Body.Bytes(), &peersResp); err != nil {
		t.Fatalf("unmarshal peers: %v", err)
	}
	if peersResp.Total != 2 {
		t.Fatalf("expected 2 peers for school2, got %d (%+v)", peersResp.Total, peersResp.Data)
	}
	for _, p := range peersResp.Data {
		if p.ID != "dev-s2-a" && p.ID != "dev-s2-b" {
			t.Fatalf("leaked non-school2 peer: %s", p.ID)
		}
	}

	// Customer queries /api/device-group/accessible -> must ONLY see School 2 groups
	h.DB.Create(&model.DeviceGroup{SchoolID: school3.ID, Name: "二中机房A"})
	wGrps := httptest.NewRecorder()
	reqGrps := httptest.NewRequest(http.MethodGet, "/api/device-group/accessible", nil)
	reqGrps.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	r.ServeHTTP(wGrps, reqGrps)
	if wGrps.Code != http.StatusOK {
		t.Fatalf("accessible groups failed: %d", wGrps.Code)
	}
	var grpsResp struct {
		Total int `json:"total"`
		Data  []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(wGrps.Body.Bytes(), &grpsResp); err != nil {
		t.Fatalf("unmarshal accessible groups: %v", err)
	}
	if grpsResp.Total != 1 || grpsResp.Data[0].Name != "默认机房" {
		t.Fatalf("expected only School 2 default group, got %+v", grpsResp)
	}

	// Customer queries /api/users -> must ONLY see School 2 users
	wUsers := httptest.NewRecorder()
	reqUsers := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	reqUsers.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	r.ServeHTTP(wUsers, reqUsers)
	if wUsers.Code != http.StatusOK {
		t.Fatalf("users failed: %d", wUsers.Code)
	}
	var usersResp struct {
		Total int `json:"total"`
		Data  []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(wUsers.Body.Bytes(), &usersResp); err != nil {
		t.Fatalf("unmarshal users: %v", err)
	}
	if usersResp.Total != 1 || usersResp.Data[0].Name != "sy_teacher" {
		t.Fatalf("expected only sy_teacher in school2 users, got %+v", usersResp)
	}

	// Super-admin deletes group 2 and group 1 -> both should succeed cleanly
	wDel2 := httptest.NewRecorder()
	reqDel2 := httptest.NewRequest(http.MethodDelete, "/api/admin/groups/2", nil)
	reqDel2.Header.Set("Authorization", "Bearer "+adminToken)
	r.ServeHTTP(wDel2, reqDel2)
	if wDel2.Code != http.StatusOK {
		t.Fatalf("delete group 2 failed: %d %s", wDel2.Code, wDel2.Body.String())
	}

	wDel1 := httptest.NewRecorder()
	reqDel1 := httptest.NewRequest(http.MethodDelete, "/api/admin/groups/1", nil)
	reqDel1.Header.Set("Authorization", "Bearer "+adminToken)
	r.ServeHTTP(wDel1, reqDel1)
	if wDel1.Code != http.StatusOK {
		t.Fatalf("delete group 1 failed: %d %s", wDel1.Code, wDel1.Body.String())
	}

	var devS1 model.Device
	h.DB.Where("id = ?", "dev-s1").First(&devS1)
	if devS1.GroupID != 0 {
		t.Fatalf("expected dev-s1 group_id=0 after deleting last group of school 1, got %d", devS1.GroupID)
	}

	// Empty username login must be rejected cleanly
	emptyLoginBody, _ := json.Marshal(map[string]interface{}{"username": "", "password": "pass1234"})
	wEmpty := httptest.NewRecorder()
	reqEmpty := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(emptyLoginBody))
	reqEmpty.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wEmpty, reqEmpty)
	if !bytes.Contains(wEmpty.Body.Bytes(), []byte("error")) {
		t.Fatalf("expected error on empty username login, got %s", wEmpty.Body.String())
	}

	// /api/ab/personal must return guid=null so client falls back to legacy /api/ab
	wAbPersonal := httptest.NewRecorder()
	reqAbPersonal := httptest.NewRequest(http.MethodPost, "/api/ab/personal", nil)
	reqAbPersonal.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	r.ServeHTTP(wAbPersonal, reqAbPersonal)
	var abPersonalResp map[string]interface{}
	if err := json.Unmarshal(wAbPersonal.Body.Bytes(), &abPersonalResp); err != nil {
		t.Fatalf("unmarshal ab/personal: %v", err)
	}
	if val, ok := abPersonalResp["guid"]; !ok || val != nil {
		t.Fatalf("expected guid=null in ab/personal, got %+v", abPersonalResp)
	}

	// Updating device group_id to 0 (ungrouped) must persist
	ungroupBody, _ := json.Marshal(map[string]interface{}{"group_id": 0})
	wUngroup := httptest.NewRecorder()
	reqUngroup := httptest.NewRequest(http.MethodPut, "/api/admin/devices/dev-s2-a", bytes.NewReader(ungroupBody))
	reqUngroup.Header.Set("Content-Type", "application/json")
	reqUngroup.Header.Set("Authorization", "Bearer "+adminToken)
	r.ServeHTTP(wUngroup, reqUngroup)
	if wUngroup.Code != http.StatusOK {
		t.Fatalf("ungroup device failed: %d %s", wUngroup.Code, wUngroup.Body.String())
	}
	var devS2A model.Device
	h.DB.Where("id = ?", "dev-s2-a").First(&devS2A)
	if devS2A.GroupID != 0 {
		t.Fatalf("expected dev-s2-a group_id=0, got %d", devS2A.GroupID)
	}

	// Moving a group across schools when name collides must NOT mutate device school_id
	var dupGrp model.DeviceGroup
	if err := h.DB.Where("school_id = ? AND name = ?", school3.ID, "默认机房").First(&dupGrp).Error; err != nil {
		t.Fatalf("find dupGrp: %v", err)
	}
	moveBody, _ := json.Marshal(map[string]interface{}{"school_id": school2.ID})
	wMove := httptest.NewRecorder()
	reqMove := httptest.NewRequest(http.MethodPut, "/api/admin/groups/"+strconv.Itoa(int(dupGrp.ID)), bytes.NewReader(moveBody))
	reqMove.Header.Set("Content-Type", "application/json")
	reqMove.Header.Set("Authorization", "Bearer "+adminToken)
	r.ServeHTTP(wMove, reqMove)
	if wMove.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on colliding group move, got %d %s", wMove.Code, wMove.Body.String())
	}
	var devS3 model.Device
	h.DB.Where("id = ?", "dev-s3").First(&devS3)
	if devS3.SchoolID != school3.ID {
		t.Fatalf("expected dev-s3 to stay in school3 after failed group move, got %d", devS3.SchoolID)
	}
}

func TestDeletedGroupsStayDeletedAcrossRestart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "restart.db")
	db, secret, err := model.InitDB(dbPath, "admin", "polaris2026", "test-secret")
	if err != nil {
		t.Fatalf("initial InitDB failed: %v", err)
	}
	var initialGroupCount int64
	db.Model(&model.DeviceGroup{}).Count(&initialGroupCount)
	if initialGroupCount != 2 {
		t.Fatalf("expected 2 seeded groups on initial bootstrap, got %d", initialGroupCount)
	}

	cfg := config.Load()
	cfg.RendezvousPort = 0
	cfg.ServerPublicHost = ""
	h := New(db, cfg, secret)
	r := gin.New()
	admin := r.Group("/api/admin")
	admin.Use(auth.RequireUser(db, secret))
	admin.DELETE("/groups/:id", h.HandleDeleteGroup)

	var adminUser model.User
	if err := db.Where("username = ?", "admin").First(&adminUser).Error; err != nil {
		t.Fatalf("find admin: %v", err)
	}
	adminToken, err := auth.GenerateToken(secret, &adminUser, 3600e9)
	if err != nil {
		t.Fatalf("admin token: %v", err)
	}

	for _, id := range []string{"1", "2"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodDelete, "/api/admin/groups/"+id, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("delete group %s failed: %d %s", id, w.Code, w.Body.String())
		}
	}

	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}

	// Simulate backend restart against the same database
	db2, _, err := model.InitDB(dbPath, "admin", "polaris2026", "test-secret")
	if err != nil {
		t.Fatalf("restart InitDB failed: %v", err)
	}
	defer func() {
		if s, err := db2.DB(); err == nil {
			_ = s.Close()
		}
	}()

	var afterRestartCount int64
	db2.Model(&model.DeviceGroup{}).Count(&afterRestartCount)
	if afterRestartCount != 0 {
		t.Fatalf("expected deleted groups to stay deleted after restart, got %d", afterRestartCount)
	}
}

func TestBytesCodecAndHbbsProbeOnline(t *testing.T) {
	// Test BytesCodec framing roundtrip for short & medium payloads
	for _, size := range []int{0, 12, 63, 64, 300, 16384} {
		payload := bytes.Repeat([]byte{0x5a}, size)
		frame, err := encodeBytesCodecFrame(payload)
		if err != nil {
			t.Fatalf("encodeBytesCodecFrame(%d) failed: %v", size, err)
		}
		got, err := readBytesCodecFrame(bytes.NewReader(frame))
		if err != nil {
			t.Fatalf("readBytesCodecFrame(%d) failed: %v", size, err)
		}
		if !bytes.Equal(got, payload) {
			t.Fatalf("roundtrip mismatch for size %d", size)
		}
	}

	// Test mock hbbs TCP port 21115 OnlineRequest / OnlineResponse
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, err = readBytesCodecFrame(conn)
		if err != nil {
			return
		}
		// Send an initial KeyExchange frame (field 25) followed by OnlineResponse (field 24)
		kxFrame, _ := encodeBytesCodecFrame(appendProtoLenField(nil, 25, []byte{0x01}))
		_, _ = conn.Write(kxFrame)
		// 10 peers -> 2 bytes: bit 7 (peer 0)=1, bit 5 (peer 2)=1, byte1 bit 6 (peer 9)=1 => [0xa0, 0x40]
		respBody := appendProtoLenField(nil, 1, []byte{0xa0, 0x40})
		respMsg := appendProtoLenField(nil, 24, respBody)
		respFrame, _ := encodeBytesCodecFrame(respMsg)
		_, _ = conn.Write(respFrame)
	}()

	ids := []string{"d0", "d1", "d2", "d3", "d4", "d5", "d6", "d7", "d8", "d9"}
	states, _, err := probeHbbsAddr(ln.Addr().String(), ids, 2*time.Second)
	if err != nil {
		t.Fatalf("probeHbbsAddr failed: %v", err)
	}
	if !states["d0"] || states["d1"] || !states["d2"] || states["d8"] || !states["d9"] {
		t.Fatalf("unexpected bitmask decode result: %+v", states)
	}

	// Verify HandleProbeDevices respects school isolation & combines live hbbs status
	h, r := setupTestHandler(t)
	liveLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen liveLn: %v", err)
	}
	defer liveLn.Close()
	_, p2Str, _ := net.SplitHostPort(liveLn.Addr().String())
	p2Num, _ := strconv.Atoi(p2Str)
	h.Cfg.RendezvousPort = p2Num + 1

	go func() {
		for {
			conn, err := liveLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				if _, err := readBytesCodecFrame(c); err != nil {
					return
				}
				respBody := appendProtoLenField(nil, 1, []byte{0x80})
				respMsg := appendProtoLenField(nil, 24, respBody)
				frame, _ := encodeBytesCodecFrame(respMsg)
				_, _ = c.Write(frame)
			}(conn)
		}
	}()

	h.DB.Create(&model.School{ID: 2, Name: "二中", Code: "ez", EnrollToken: "ez-tok", Status: 1})
	h.DB.Create(&model.Device{ID: "agent-s1", ClientRole: "agent", SchoolID: 1, LastSeenAt: time.Now()})
	h.DB.Create(&model.Device{ID: "agent-s2", ClientRole: "agent", SchoolID: 2, LastSeenAt: time.Now().Add(-10 * time.Minute)})

	var adminUser model.User
	h.DB.Where("username = ?", "admin").First(&adminUser)
	adminToken, _ := auth.GenerateToken(h.JWTSecret, &adminUser, time.Hour)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/devices/probe", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("HandleProbeDevices failed: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Total         int `json:"total"`
		OnlineCount   int `json:"online_count"`
		HbbsReachable bool `json:"hbbs_reachable"`
		States        map[string]struct {
			Online     bool  `json:"online"`
			HbbsOnline *bool `json:"hbbs_online"`
		} `json:"states"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode probe resp: %v", err)
	}
	if !out.HbbsReachable || out.Total != 2 || out.OnlineCount != 1 || !out.States["agent-s1"].Online || out.States["agent-s2"].Online {
		t.Fatalf("unexpected HandleProbeDevices result: %+v", out)
	}
}
