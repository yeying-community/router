package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	channel "github.com/yeying-community/router/internal/admin/controller/channel"
	"github.com/yeying-community/router/internal/admin/model"
)

const (
	asyncTaskWorkerCount                    = 2
	asyncTaskPollInterval                   = 1200 * time.Millisecond
	runtimeCapabilityRecoveryProbeInterval  = 10 * time.Minute
	runtimeCapabilityRecoveryProbeBatchSize = 100
)

var startAsyncTaskWorkersOnce sync.Once
var runningAsyncTaskCancels sync.Map

func registerRunningAsyncTaskCancel(taskID string, cancel context.CancelFunc) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || cancel == nil {
		return
	}
	runningAsyncTaskCancels.Store(taskID, cancel)
}

func unregisterRunningAsyncTaskCancel(taskID string) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return
	}
	runningAsyncTaskCancels.Delete(taskID)
}

func CancelRunningTask(taskID string) bool {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return false
	}
	value, ok := runningAsyncTaskCancels.Load(taskID)
	if !ok {
		return false
	}
	cancel, ok := value.(context.CancelFunc)
	if !ok || cancel == nil {
		return false
	}
	cancel()
	return true
}

func StartAsyncTaskWorkers() {
	startAsyncTaskWorkersOnce.Do(func() {
		rows, err := model.FailRunningAsyncTasksWithDB(model.DB, "任务在服务重启前未完成，已标记失败")
		if err != nil {
			logger.Warn(context.Background(), fmt.Sprintf("[async-task] recover_running_failed error=%q", err.Error()))
		} else if rows > 0 {
			logger.Info(context.Background(), fmt.Sprintf("[async-task] recovered %d stale running tasks", rows))
		}
		for idx := 0; idx < asyncTaskWorkerCount; idx++ {
			go asyncTaskWorkerLoop(idx + 1)
		}
		go runtimeCapabilityRecoveryProbeLoop()
	})
}

func runtimeCapabilityRecoveryProbeLoop() {
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		<-timer.C
		created, err := channel.EnqueueRuntimeDisabledCapabilityRecoveryTests(runtimeCapabilityRecoveryProbeBatchSize)
		if err != nil {
			logger.Warn(context.Background(), fmt.Sprintf("[async-task] runtime_capability_recovery_probe_failed error=%q", err.Error()))
		} else if created > 0 {
			logger.Info(context.Background(), fmt.Sprintf("[async-task] runtime_capability_recovery_probe_enqueued count=%d", created))
		}
		timer.Reset(runtimeCapabilityRecoveryProbeInterval)
	}
}

func asyncTaskWorkerLoop(workerIndex int) {
	for {
		taskRow, err := model.ClaimNextPendingAsyncTaskWithDB(model.DB)
		if err != nil {
			logger.Warn(context.Background(), fmt.Sprintf("[async-task] worker=%d claim_failed error=%q", workerIndex, err.Error()))
			time.Sleep(asyncTaskPollInterval)
			continue
		}
		if taskRow == nil {
			time.Sleep(asyncTaskPollInterval)
			continue
		}
		ctx := context.Background()
		if traceID := strings.TrimSpace(taskRow.TraceID); traceID != "" {
			ctx = helper.SetTraceID(ctx, traceID)
		}
		execCtx, cancel := context.WithCancel(ctx)
		registerRunningAsyncTaskCancel(taskRow.Id, cancel)
		logger.Info(ctx, fmt.Sprintf("[async-task] worker=%d task_id=%s type=%s status=running", workerIndex, taskRow.Id, taskRow.Type))
		result, execErr := channel.ExecuteAsyncTask(execCtx, taskRow)
		unregisterRunningAsyncTaskCancel(taskRow.Id)
		cancel()
		finalStatus := model.AsyncTaskStatusSucceeded
		errorMessage := ""
		if execErr != nil {
			if errors.Is(execErr, context.Canceled) {
				finalStatus = model.AsyncTaskStatusCanceled
				errorMessage = "任务已取消"
			} else {
				finalStatus = model.AsyncTaskStatusFailed
				errorMessage = execErr.Error()
			}
		} else if resolvedStatus, resolvedMessage, ok := model.ResolveAsyncTaskBusinessOutcome(taskRow.Type, result); ok {
			finalStatus = resolvedStatus
			if finalStatus != model.AsyncTaskStatusSucceeded {
				errorMessage = resolvedMessage
			}
		}
		finishErr := model.FinishAsyncTaskWithDB(model.DB, taskRow.Id, finalStatus, result, errorMessage)
		if finishErr != nil {
			logger.Warn(ctx, fmt.Sprintf("[async-task] worker=%d task_id=%s finish_failed error=%q", workerIndex, taskRow.Id, finishErr.Error()))
			time.Sleep(asyncTaskPollInterval)
			continue
		}
		if execErr != nil {
			if finalStatus == model.AsyncTaskStatusCanceled {
				logger.Info(ctx, fmt.Sprintf("[async-task] worker=%d task_id=%s type=%s status=canceled", workerIndex, taskRow.Id, taskRow.Type))
			} else {
				logger.Warn(ctx, fmt.Sprintf("[async-task] worker=%d task_id=%s type=%s status=failed error=%q", workerIndex, taskRow.Id, taskRow.Type, execErr.Error()))
			}
		} else {
			logger.Info(ctx, fmt.Sprintf("[async-task] worker=%d task_id=%s type=%s status=succeeded", workerIndex, taskRow.Id, taskRow.Type))
		}
	}
}
