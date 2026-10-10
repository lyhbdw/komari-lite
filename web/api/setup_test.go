package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/database/models"
)

func newSetupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/setup/status", SetupStatus)
	r.POST("/api/setup/create-admin", SetupCreateAdmin)
	return r
}

func setupRequest(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := newSetupRouter()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func setupTestDB(t *testing.T) {
	t.Helper()
	db := dbcore.OpenTestDB(t)
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	dbcore.SwapInstance(t, db)
}

func TestSetupStatusRequiresSetupOnEmptyDB(t *testing.T) {
	setupTestDB(t)
	w := setupRequest(t, "GET", "/api/setup/status", "")
	var resp struct {
		Data struct {
			SetupRequired bool `json:"setup_required"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad response: %v", w.Body.String())
	}
	if w.Code != http.StatusOK || !resp.Data.SetupRequired {
		t.Fatalf("expected setup_required=true on empty db, got %d %s", w.Code, w.Body.String())
	}
}

func TestSetupCreateAdminThenSelfCloses(t *testing.T) {
	setupTestDB(t)

	// create first admin
	w := setupRequest(t, "POST", "/api/setup/create-admin",
		`{"username":"admin","password":"password123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on first create, got %d %s", w.Code, w.Body.String())
	}

	// status now reports setup not required
	w = setupRequest(t, "GET", "/api/setup/status", "")
	var resp struct {
		Data struct {
			SetupRequired bool `json:"setup_required"`
		} `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Data.SetupRequired {
		t.Fatal("expected setup_required=false after account exists")
	}

	// second create attempt must be forbidden
	w = setupRequest(t, "POST", "/api/setup/create-admin",
		`{"username":"evil","password":"password456"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 on second create, got %d %s", w.Code, w.Body.String())
	}

	// short password rejected
	w = setupRequest(t, "POST", "/api/setup/create-admin",
		`{"username":"x","password":"short"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for short password, got %d", w.Code)
	}
}
