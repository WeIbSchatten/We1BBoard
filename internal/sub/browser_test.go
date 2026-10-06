package sub

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWantsSubPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name     string
		ua       string
		accept   string
		secDest  string
		query    string
		want     bool
		full     bool
	}{
		{"chrome navigate", "Mozilla/5.0 Chrome/120", "text/html", "document", "", true, false},
		{"explicit html", "curl/8", "", "", "html=1", true, true},
		{"view html", "curl/8", "", "", "view=html", true, true},
		{"clash client", "ClashMeta/1.0 Mozilla/5.0", "*/*", "", "", false, false},
		{"sing-box", "sing-box/1.8", "*/*", "", "", false, false},
		{"v2rayN", "v2rayN/6 Mozilla/5.0", "*/*", "", "", false, false},
		{"curl raw", "curl/8.0", "*/*", "", "", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodGet, "/sub/abc?"+tc.query, nil)
			req.Header.Set("User-Agent", tc.ua)
			if tc.accept != "" {
				req.Header.Set("Accept", tc.accept)
			}
			if tc.secDest != "" {
				req.Header.Set("Sec-Fetch-Dest", tc.secDest)
				req.Header.Set("Sec-Fetch-Mode", "navigate")
			}
			c.Request = req
			want, full := wantsSubPage(c)
			if want != tc.want || full != tc.full {
				t.Fatalf("got want=%v full=%v, want want=%v full=%v", want, full, tc.want, tc.full)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	if formatBytes(0) != "0 B" {
		t.Fatal(formatBytes(0))
	}
	if formatBytes(1536) != "1.50 KB" {
		t.Fatal(formatBytes(1536))
	}
}
