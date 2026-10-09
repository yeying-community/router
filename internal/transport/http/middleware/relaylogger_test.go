package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/ctxkey"
)

func TestShouldWarnForUnavailableModel(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		modelFlag     bool
		errorCode     string
		wantWarnLevel bool
	}{
		{
			name:          "retired model entitlement failure",
			status:        http.StatusForbidden,
			modelFlag:     true,
			errorCode:     "entitlement_unavailable",
			wantWarnLevel: true,
		},
		{
			name:          "user entitlement failure",
			status:        http.StatusForbidden,
			modelFlag:     false,
			errorCode:     "entitlement_unavailable",
			wantWarnLevel: false,
		},
		{
			name:          "different error code",
			status:        http.StatusForbidden,
			modelFlag:     true,
			errorCode:     "token_quota_exhausted",
			wantWarnLevel: false,
		},
		{
			name:          "unpublished model",
			status:        http.StatusForbidden,
			modelFlag:     true,
			errorCode:     "model_not_found",
			wantWarnLevel: true,
		},
		{
			name:          "different status",
			status:        http.StatusInternalServerError,
			modelFlag:     true,
			errorCode:     "entitlement_unavailable",
			wantWarnLevel: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set(ctxkey.RelayModelUnavailable, tt.modelFlag)
			c.Set(ctxkey.RelayErrorCode, tt.errorCode)
			if got := shouldWarnForUnavailableModel(c, tt.status); got != tt.wantWarnLevel {
				t.Fatalf("shouldWarnForUnavailableModel()=%v, want %v", got, tt.wantWarnLevel)
			}
		})
	}
}
