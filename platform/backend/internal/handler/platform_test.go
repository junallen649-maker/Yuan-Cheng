package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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
	h := New(db, cfg, secret)

	r := gin.New()
	r.POST("/api/sysinfo", h.HandleSysinfo)
	r.POST("/api/heartbeat", h.HandleHeartbeat)
	r.POST("/api/command/ack", h.HandleCommandAck)
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
