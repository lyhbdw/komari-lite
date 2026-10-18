package public

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lyhbdw/komari-lite/database/dbcore"
	"github.com/lyhbdw/komari-lite/internal/config"
)

func TestNormalizeHTMLLanguage(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"hyphen language": {
			input: "zh-CN",
			want:  "zh-CN",
		},
		"underscore language": {
			input: "zh_CN",
			want:  "zh-CN",
		},
		"reject script injection": {
			input: `zh-CN" autofocus`,
		},
		"reject too short": {
			input: "z",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := normalizeHTMLLanguage(tt.input); got != tt.want {
				t.Fatalf("normalizeHTMLLanguage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestReplaceHTMLLanguage(t *testing.T) {
	tests := map[string]struct {
		html     string
		language string
		want     string
	}{
		"replace existing lang": {
			html:     `<html lang="en"><head></head></html>`,
			language: "zh-CN",
			want:     `<html lang="zh-CN"><head></head></html>`,
		},
		"insert missing lang": {
			html:     `<html><head></head></html>`,
			language: "ja_JP",
			want:     `<html lang="ja-JP"><head></head></html>`,
		},
		"ignore invalid lang": {
			html:     `<html lang="en"><head></head></html>`,
			language: `zh-CN" autofocus`,
			want:     `<html lang="en"><head></head></html>`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := replaceHTMLLanguage(tt.html, tt.language); got != tt.want {
				t.Fatalf("replaceHTMLLanguage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEmbeddedDistDoesNotEmbedRawFiles(t *testing.T) {
	if _, err := PublicFS.ReadFile("defaultTheme/dist/index.html"); err == nil {
		t.Fatal("PublicFS still embeds the raw frontend files")
	}
	if content, ok := defaultDistFiles[IndexFile]; !ok || len(content) == 0 {
		t.Fatalf("embedded dist does not contain a non-empty %q", IndexFile)
	}
}

func TestServeIndexWithAdminReferer(t *testing.T) {
	db := dbcore.OpenTestDB(t)
	if err := config.SetDb(db); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Static(engine.Group("/"), func(handlers ...gin.HandlerFunc) {
		engine.NoRoute(handlers...)
	})

	// 1. GET / with Referer /admin should serve Lite theme, NOT __admin__
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Referer", "https://komari.beaver1376.top/admin")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, `og:title" content="Monitor"`) {
		t.Fatalf("GET / with admin referer served __admin__ theme instead of default Lite theme: %s", body)
	}

	// 2. GET /admin should serve __admin__ theme
	reqAdmin := httptest.NewRequest(http.MethodGet, "/admin", nil)
	wAdmin := httptest.NewRecorder()
	engine.ServeHTTP(wAdmin, reqAdmin)
	if wAdmin.Code != http.StatusOK {
		t.Fatalf("expected 200 for /admin, got %d", wAdmin.Code)
	}
	bodyAdmin := wAdmin.Body.String()
	if !strings.Contains(bodyAdmin, `<div id="root"></div>`) {
		t.Fatalf("expected /admin to serve __admin__ SPA (root div): %s", bodyAdmin)
	}

	// 3. HEAD / should return 200
	reqHead := httptest.NewRequest(http.MethodHead, "/", nil)
	wHead := httptest.NewRecorder()
	engine.ServeHTTP(wHead, reqHead)
	if wHead.Code != http.StatusOK {
		t.Fatalf("expected 200 for HEAD /, got %d", wHead.Code)
	}

	// 4. Missing asset under /assets/ should return 404, not 200 HTML
	reqMissingAsset := httptest.NewRequest(http.MethodGet, "/assets/not-found-12345.js", nil)
	wMissingAsset := httptest.NewRecorder()
	engine.ServeHTTP(wMissingAsset, reqMissingAsset)
	if wMissingAsset.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing asset, got %d", wMissingAsset.Code)
	}
}
