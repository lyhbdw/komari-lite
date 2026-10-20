package public

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/internal/config"
	"gorm.io/gorm"
)

func staticTestEngine(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	t.Chdir(t.TempDir())
	db := dbcore.OpenTestDB(t)
	if err := config.SetDb(db); err != nil {
		t.Fatal(err)
	}
	items := []config.ConfigItem{
		{Key: config.SitenameKey, Value: `"Custom Monitor"`},
		{Key: config.DescriptionKey, Value: `"Custom description"`},
		{Key: config.ThemeKey, Value: `"Other"`},
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	oldDefault, oldAdmin := defaultDistFiles, adminDistFiles
	t.Cleanup(func() { defaultDistFiles, adminDistFiles = oldDefault, oldAdmin })
	defaultDistFiles = map[string][]byte{
		IndexFile:               []byte(`<html lang="en"><head><title>Komari Monitor</title><meta name="description" content="A simple server monitor tool."></head><body>public<script id="vite-plugin-pwa:register-sw" src="/registerSW.js"></script></body></html>`),
		"assets/shared.js":      []byte("public asset"),
		"assets/public-only.js": []byte("public only"),
		"favicon.ico":           []byte("public ico"),
		"favicon.svg":           []byte("public svg"),
		"manifest.webmanifest":  []byte(`{"name":"public"}`),
	}
	adminDistFiles = map[string][]byte{
		IndexFile:              []byte(`<html lang="en"><head><title>Monitor</title></head><body>admin<script id="vite-plugin-pwa:register-sw" src="/registerSW.js"></script></body></html>`),
		"assets/shared.js":     []byte("admin asset"),
		"assets/admin-only.js": []byte("admin only"),
		"favicon.ico":          []byte("admin ico"),
		"apple-touch-icon.png": []byte("admin apple"),
		"manifest.webmanifest": []byte(`{"name":"admin"}`),
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	Static(r.Group("/"), func(handlers ...gin.HandlerFunc) { r.NoRoute(handlers...) })
	return r, db
}

func writeStaticTestFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestStaticCompatibility(t *testing.T) {
	r, _ := staticTestEngine(t)
	writeStaticTestFile(t, "data/theme/Lite/dist/assets/shared.js", "disk public asset")
	writeStaticTestFile(t, "data/theme/__admin__/dist/assets/shared.js", "must not serve disk admin")
	writeStaticTestFile(t, "data/theme/Other/dist/assets/shared.js", "must not serve configured theme")
	writeStaticTestFile(t, "data/private.txt", "must not leak")
	for _, tt := range []struct {
		name, method, path, referer, want string
		status                            int
	}{
		{"Lite disk override", "GET", "/assets/shared.js", "", "disk public asset", 200},
		{"admin referrer takes priority", "GET", "/assets/shared.js", "https://example.com/admin/settings", "admin asset", 200},
		{"admin path takes priority", "GET", "/admin/../assets/shared.js", "", "admin asset", 200},
		{"public asset falls back to admin", "GET", "/assets/admin-only.js", "", "admin only", 200},
		{"admin asset falls back to Lite", "GET", "/assets/public-only.js", "https://example.com/admin", "public only", 200},
		{"explicit Lite", "GET", "/themes/Lite/dist/assets/shared.js", "https://example.com/admin", "disk public asset", 200},
		{"explicit admin is embedded", "GET", "/themes/__admin__/dist/assets/shared.js", "", "admin asset", 200},
		{"other themes stay disabled", "GET", "/themes/Other/dist/assets/shared.js", "", "", 404},
		{"embedded metadata", "GET", "/themes/Lite/komari-theme.json", "", "", 200},
		{"manifest", "GET", "/manifest.webmanifest", "", `{"name":"public"}`, 200},
		{"admin manifest", "GET", "/manifest.webmanifest", "https://example.com/admin", `{"name":"admin"}`, 200},
		{"ico", "GET", "/favicon.ico", "https://example.com/admin", "public ico", 200},
		{"svg", "GET", "/favicon.svg", "https://example.com/admin", "public svg", 200},
		{"apple admin fallback", "GET", "/apple-touch-icon.png", "", "admin apple", 200},
		{"HEAD icon", "HEAD", "/favicon.svg", "", "", 200},
		{"missing asset", "GET", "/assets/missing.js", "", "", 404},
		{"POST cannot fall back to SPA", "POST", "/instance/example", "", "", 404},
		{"Lite traversal", "GET", "/themes/Lite/../../private.txt", "", "", 404},
		{"encoded traversal", "GET", "/themes/Lite/%2e%2e/%2e%2e/private.txt", "", "", 404},
		{"admin traversal", "GET", "/themes/__admin__/../../private.txt", "", "", 404},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Referer", tt.referer)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.status || (tt.want != "" && w.Body.String() != tt.want) {
				t.Fatalf("%s %s: status=%d body=%q, want status=%d body=%q", tt.method, tt.path, w.Code, w.Body.String(), tt.status, tt.want)
			}
			if strings.Contains(w.Body.String(), "must not") {
				t.Fatalf("served forbidden content: %q", w.Body.String())
			}
		})
	}
}

func TestStaticIndexCustomization(t *testing.T) {
	r, _ := staticTestEngine(t)
	for _, url := range []string{"/", "/instance/example", "/admin", "/admin/settings"} {
		t.Run(url, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, url, nil)
			req.Header.Set("Referer", "https://example.com/admin")
			req.AddCookie(&http.Cookie{Name: LanguageCookieName, Value: "zh_CN"})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			body := w.Body.String()
			if w.Code != 200 || !strings.Contains(body, `<html lang="zh-CN">`) || !strings.Contains(body, `id="vite-plugin-pwa:register-sw"`) {
				t.Fatalf("index or language/PWA changed: status=%d body=%q", w.Code, body)
			}
			if strings.HasPrefix(url, "/admin") {
				if !strings.Contains(body, "<title>Monitor</title>") || strings.Contains(body, "Custom Monitor") || strings.Contains(body, "__INITIAL_SITENAME__") {
					t.Fatalf("admin index was customized: %q", body)
				}
			} else if !strings.Contains(body, "<title>Custom Monitor</title>") || !strings.Contains(body, "Custom description") || !strings.Contains(body, `window.__INITIAL_SITENAME__="Custom Monitor";`) {
				t.Fatalf("public index was not customized: %q", body)
			}
		})
	}
	writeStaticTestFile(t, "data/theme/Lite/dist/index.html", `<html><head><title>Monitor</title></head><body>disk index</body></html>`)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "disk index") || !strings.Contains(w.Body.String(), "Custom Monitor") {
		t.Fatalf("Lite disk index override changed: status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestStaticIconOverrides(t *testing.T) {
	r, _ := staticTestEngine(t)
	for _, filename := range []string{"favicon.ico", "favicon.svg", "apple-touch-icon.png"} {
		writeStaticTestFile(t, filepath.Join(DataDir, filename), "custom "+filename)
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/"+filename, nil)
			r.ServeHTTP(w, req)
			if w.Code != 200 || (method == http.MethodGet && w.Body.String() != "custom "+filename) {
				t.Fatalf("%s %s override changed: status=%d body=%q", method, filename, w.Code, w.Body.String())
			}
		}
	}
}

func TestStaticReadsConfigOnlyForPublicIndex(t *testing.T) {
	r, db := staticTestEngine(t)
	var queries int
	var keys []string
	if err := db.Callback().Query().After("gorm:query").Register(t.Name(), func(db *gorm.DB) {
		if db.Statement.Table != "configs" {
			return
		}
		queries++
		for _, value := range db.Statement.Vars {
			if key, ok := value.(string); ok {
				keys = append(keys, key)
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		method, path, referer string
		status, wantQueries   int
	}{
		{"GET", "/assets/shared.js", "", 200, 0},
		{"GET", "/assets/shared.js", "https://example.com/admin", 200, 0},
		{"HEAD", "/assets/shared.js", "", 200, 0},
		{"GET", "/assets/missing.js", "", 404, 0},
		{"GET", "/favicon.ico", "", 200, 0},
		{"HEAD", "/favicon.ico", "", 200, 0},
		{"GET", "/favicon.svg", "", 200, 0},
		{"GET", "/apple-touch-icon.png", "", 200, 0},
		{"GET", "/manifest.webmanifest", "", 200, 0},
		{"GET", "/manifest.webmanifest", "https://example.com/admin", 200, 0},
		{"GET", "/themes/Lite/komari-theme.json", "", 200, 0},
		{"GET", "/themes/Lite/dist/index.html", "", 200, 0},
		{"GET", "/themes/Other/dist/index.html", "", 404, 0},
		{"GET", "/admin", "", 200, 0},
		{"GET", "/admin/settings", "", 200, 0},
		{"HEAD", "/admin", "", 200, 0},
		{"POST", "/instance/example", "", 404, 0},
		{"GET", "/", "https://example.com/admin", 200, 1},
		{"HEAD", "/", "", 200, 1},
		{"GET", "/instance/example", "", 200, 1},
	} {
		t.Run(tt.method+" "+tt.path+" "+tt.referer, func(t *testing.T) {
			queries, keys = 0, nil
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Referer", tt.referer)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tt.status {
				t.Fatalf("status=%d, want %d", w.Code, tt.status)
			}
			if queries != tt.wantQueries {
				t.Errorf("config queries=%d, want %d (keys=%v)", queries, tt.wantQueries, keys)
			}
			if tt.wantQueries != 0 {
				sort.Strings(keys)
				wantKeys := []string{config.DescriptionKey, config.SitenameKey}
				sort.Strings(wantKeys)
				if strings.Join(keys, ",") != strings.Join(wantKeys, ",") {
					t.Errorf("queried config keys=%v, want only %v", keys, wantKeys)
				}
			}
		})
	}
	delete(defaultDistFiles, IndexFile)
	queries = 0
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusNotFound || queries != 0 {
		t.Errorf("missing public index: status=%d config queries=%d, want 404 without config reads", w.Code, queries)
	}
}
