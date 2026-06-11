package monitor

import (
	"net/http"
	"testing"

	"github.com/yeying-community/router/common/config"
	relaymodel "github.com/yeying-community/router/internal/relay/model"
)

func TestShouldDisableChannelForZhipuInsufficientBalanceCode(t *testing.T) {
	previous := config.AutomaticDisableChannelEnabled
	config.AutomaticDisableChannelEnabled = true
	defer func() {
		config.AutomaticDisableChannelEnabled = previous
	}()

	err := &relaymodel.Error{
		Message: "余额不足或无可用资源包,请充值。",
		Code:    "1113",
	}

	if !ShouldDisableChannel(err, http.StatusTooManyRequests) {
		t.Fatalf("ShouldDisableChannel = false, want true")
	}
}
