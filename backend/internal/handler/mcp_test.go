package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CriarBrand/qianchuan-backend/internal/service"
	"github.com/gin-gonic/gin"
)

func TestMCPStatusSerializesLargeIDsAsStrings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewMCPHandler(&service.QianchuanService{
		DefaultAdvertiserID: 1855810750424170,
		DefaultAwemeID:      63307887035,
		DefaultLiveRoomID:   7709635958206991154,
	}, "test-key", false)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/mcp/status", nil)

	h.Status(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	var response struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := map[string]string{
		"advertiser_id": "1855810750424170",
		"aweme_id":      "63307887035",
		"live_room_id":  "7709635958206991154",
	}
	for field, expected := range want {
		if actual, ok := response.Data[field].(string); !ok || actual != expected {
			t.Errorf("%s = %#v, want %q", field, response.Data[field], expected)
		}
	}
	if response.Data["write_enabled"] != false {
		t.Errorf("write_enabled = %#v, want false", response.Data["write_enabled"])
	}
}
