package jsonrpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lyhbdw/komari-lite/database"
	"github.com/lyhbdw/komari-lite/database/models"
	"github.com/lyhbdw/komari-lite/internal/config"
	"github.com/lyhbdw/komari-lite/pkg/rpc"
	"github.com/lyhbdw/komari-lite/web/public"
)

func TestCleanupPublicInfoUsesDefaultTheme(t *testing.T) {
	db, _ := queryBudgetFixture(t)
	if err := db.AutoMigrate(&models.ThemeConfiguration{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&config.ConfigItem{Key: "theme", Value: `"old-custom-theme"`}).Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, data string
		present    bool
		want       map[string]any
	}{
		{"missing", "", false, map[string]any{}},
		{"empty", `{}`, true, map[string]any{}},
		{"existing", `{"title":"custom title","enabled":false}`, true, map[string]any{"title": "custom title", "enabled": false}},
		{"malformed", `{invalid`, true, map[string]any{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.Where("short = ?", public.DefaultTheme).Delete(&models.ThemeConfiguration{}).Error; err != nil {
				t.Fatal(err)
			}
			if tc.present {
				if err := db.Create(&models.ThemeConfiguration{Short: public.DefaultTheme, Data: tc.data}).Error; err != nil {
					t.Fatal(err)
				}
			}
			result, err := database.GetPublicInfo()
			if err != nil {
				t.Fatal(err)
			}
			if result["theme"] != public.DefaultTheme {
				t.Fatalf("theme=%v", result["theme"])
			}
			got, ok := result["theme_settings"].(gin.H)
			if !ok || !reflect.DeepEqual(map[string]any(got), tc.want) {
				t.Fatalf("settings=%#v want %#v", result["theme_settings"], tc.want)
			}
			if result["record_preserve_time"] != result["ping_record_preserve_time"] || result["record_enabled"] != true {
				t.Fatalf("retention changed: %#v", result)
			}
		})
	}
}

func TestCleanupBridgeResponseContract(t *testing.T) {
	for _, tc := range []struct {
		name     string
		kind     renderKind
		response *rpc.JsonRpcResponse
		code     int
		want     string
	}{
		{"standard", renderStandard, rpc.SuccessResponse(1, map[string]any{"x": 1}), http.StatusOK, `{"status":"success","message":"","data":{"x":1}}`},
		{"standard-nil", renderStandard, rpc.SuccessResponse(1, nil), http.StatusOK, `{"status":"success","message":""}`},
		{"flat", renderFlat, rpc.SuccessResponse(1, map[string]any{"x": 1}), http.StatusOK, `{"status":"success","x":1}`},
		{"raw", renderRaw, rpc.SuccessResponse(1, []int{}), http.StatusOK, `[]`},
		{"error", renderStandard, rpc.ErrorResponse(1, rpc.InvalidParams, "bad params", nil), http.StatusBadRequest, `{"status":"error","message":"bad params"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			renderResponse(c, &bindConfig{render: tc.kind}, tc.response)
			var got, want any
			if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
				t.Fatal(err)
			}
			if recorder.Code != tc.code || !reflect.DeepEqual(got, want) {
				t.Fatalf("%d %s want %d %s", recorder.Code, recorder.Body.String(), tc.code, tc.want)
			}
		})
	}
}
