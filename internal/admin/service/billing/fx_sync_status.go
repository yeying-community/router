package billing

import (
	"strconv"
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/internal/admin/model"
)

const fxSyncLastErrorMaxLength = 1024

func RecordFXSyncRun(runAt int64) {
	_ = model.UpdateOption("FXAutoSyncLastRunAt", strconv.FormatInt(runAt, 10))
}

func RecordFXSyncSuccess(runAt int64) {
	_ = model.UpdateOption("FXAutoSyncLastSuccessAt", strconv.FormatInt(runAt, 10))
	_ = model.UpdateOption("FXAutoSyncLastError", "")
	_ = model.UpdateOption("FXAutoSyncConsecutiveFailures", "0")
}

func RecordFXSyncFailure(err error) string {
	message := normalizeFXSyncErrorMessage(err)
	nextFailures := config.FXAutoSyncConsecutiveFailures + 1
	_ = model.UpdateOption("FXAutoSyncLastError", message)
	_ = model.UpdateOption("FXAutoSyncConsecutiveFailures", strconv.Itoa(nextFailures))
	return message
}

func normalizeFXSyncErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) > fxSyncLastErrorMaxLength {
		message = message[:fxSyncLastErrorMaxLength]
	}
	return message
}
