package monitor

import (
	"fmt"
	"html"
	"strings"

	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/logger"
	"github.com/yeying-community/router/common/message"
	"github.com/yeying-community/router/internal/admin/model"
)

func notifyRootUser(subject string, content string) error {
	if strings.TrimSpace(config.NotifyProvider) != "" && strings.TrimSpace(config.NotifyWebhookURL) != "" {
		err := message.SendMessage(subject, content, content)
		if err != nil {
			logger.SysError(fmt.Sprintf("failed to send message: %s", err.Error()))
		} else {
			return nil
		}
	}
	if config.RootUserEmail == "" {
		config.RootUserEmail = model.GetRootUserEmail()
	}
	err := message.SendEmail(subject, config.RootUserEmail, content)
	if err != nil {
		logger.SysError(fmt.Sprintf("failed to send email: %s", err.Error()))
	}
	return err
}

func NotifyRootUser(subject string, content string) error {
	return notifyRootUser(subject, content)
}

func NotifyChannelModelCapabilityDisabled(channelId string, channelName string, modelName string, reason string) {
	subject := "渠道模型能力摘除提醒"
	content := message.EmailTemplate(
		subject,
		fmt.Sprintf(`
			<p>您好！</p>
			<p>渠道「<strong>%s</strong>」（#%s）的模型能力已被系统自动摘除。</p>
			<p>模型：<strong>%s</strong></p>
			<p>摘除原因：</p>
			<p style="background-color: #f8f8f8; padding: 10px; border-radius: 4px;">%s</p>
			<p>该模型已从运行态路由候选中移除，请检查上游模型权限或模型名称配置。</p>
		`, notificationValue(channelName), notificationValue(channelId), notificationValue(modelName), notificationValue(reason)),
	)
	_ = notifyRootUser(subject, content)
}

func NotifyChannelModelEndpointCapabilityDisabled(channelId string, channelName string, modelName string, endpoint string, reason string) {
	subject := "渠道模型端点能力摘除提醒"
	content := message.EmailTemplate(
		subject,
		fmt.Sprintf(`
			<p>您好！</p>
			<p>渠道「<strong>%s</strong>」（#%s）的模型端点能力已被系统自动摘除。</p>
			<p>模型：<strong>%s</strong></p>
			<p>端点：<strong>%s</strong></p>
			<p>摘除原因：</p>
			<p style="background-color: #f8f8f8; padding: 10px; border-radius: 4px;">%s</p>
			<p>该模型端点已从运行态路由候选中移除，请检查供应商端点支持情况或渠道配置。</p>
		`, notificationValue(channelName), notificationValue(channelId), notificationValue(modelName), notificationValue(endpoint), notificationValue(reason)),
	)
	_ = notifyRootUser(subject, content)
}

func NotifyChannelModelCapabilityRestored(channelId string, channelName string, modelName string, operator string) {
	subject := "渠道模型能力恢复提醒"
	content := message.EmailTemplate(
		subject,
		fmt.Sprintf(`
			<p>您好！</p>
			<p>渠道「<strong>%s</strong>」（#%s）的模型能力已恢复。</p>
			<p>模型：<strong>%s</strong></p>
			<p>操作人：<strong>%s</strong></p>
			<p>该模型已重新进入运行态路由候选，请确认上游模型权限和计费配置符合预期。</p>
		`, notificationValue(channelName), notificationValue(channelId), notificationValue(modelName), notificationValue(operator)),
	)
	_ = notifyRootUser(subject, content)
}

func NotifyChannelModelEndpointCapabilityRestored(channelId string, channelName string, modelName string, endpoint string, operator string) {
	subject := "渠道模型端点能力恢复提醒"
	content := message.EmailTemplate(
		subject,
		fmt.Sprintf(`
			<p>您好！</p>
			<p>渠道「<strong>%s</strong>」（#%s）的模型端点能力已恢复。</p>
			<p>模型：<strong>%s</strong></p>
			<p>端点：<strong>%s</strong></p>
			<p>操作人：<strong>%s</strong></p>
			<p>该模型端点已重新进入运行态路由候选，请确认上游端点支持情况和测试结果符合预期。</p>
		`, notificationValue(channelName), notificationValue(channelId), notificationValue(modelName), notificationValue(endpoint), notificationValue(operator)),
	)
	_ = notifyRootUser(subject, content)
}

func notificationValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "未提供"
	}
	return html.EscapeString(value)
}

// DisableChannel disable & notify
func DisableChannel(channelId string, channelName string, reason string) {
	_ = model.RecordChannelCircuitBreakerCanceled(channelId, reason)
	model.UpdateChannelStatusById(channelId, model.ChannelStatusAutoDisabled)
	logger.SysLog(fmt.Sprintf("channel #%s has been disabled: %s", channelId, reason))
	subject := fmt.Sprintf("渠道状态变更提醒")
	content := message.EmailTemplate(
		subject,
		fmt.Sprintf(`
			<p>您好！</p>
			<p>渠道「<strong>%s</strong>」（#%s）已被禁用。</p>
			<p>禁用原因：</p>
			<p style="background-color: #f8f8f8; padding: 10px; border-radius: 4px;">%s</p>
		`, channelName, channelId, reason),
	)
	_ = notifyRootUser(subject, content)
}

func DisableChannelForInsufficientBalance(channelId string, channelName string, balance float64) {
	_ = model.RecordChannelCircuitBreakerCanceled(channelId, "insufficient balance")
	model.UpdateChannelStatusById(channelId, model.ChannelStatusAutoDisabled)
	logger.SysLog(fmt.Sprintf("channel #%s has been disabled due to insufficient balance: %.4f", channelId, balance))
	subject := fmt.Sprintf("渠道余额不足提醒")
	content := message.EmailTemplate(
		subject,
		fmt.Sprintf(`
			<p>您好！</p>
			<p>渠道「<strong>%s</strong>」（#%s）定时刷新账务后发现余额不足，已被系统自动禁用。</p>
			<p>当前余额：</p>
			<p style="background-color: #f8f8f8; padding: 10px; border-radius: 4px;"><strong>%.4f</strong></p>
			<p>请及时检查上游账户余额或补充采购记录。</p>
		`, channelName, channelId, balance),
	)
	_ = notifyRootUser(subject, content)
}

func MetricDisableChannel(channelId string, successRate float64) {
	model.UpdateChannelStatusById(channelId, model.ChannelStatusAutoDisabled)
	logger.SysLog(fmt.Sprintf("channel #%s has been disabled due to low success rate: %.2f", channelId, successRate*100))
	subject := fmt.Sprintf("渠道状态变更提醒")
	content := message.EmailTemplate(
		subject,
		fmt.Sprintf(`
			<p>您好！</p>
			<p>渠道 #%s 已被系统自动禁用。</p>
			<p>禁用原因：</p>
			<p style="background-color: #f8f8f8; padding: 10px; border-radius: 4px;">该渠道在最近 %d 次调用中成功率为 <strong>%.2f%%</strong>，低于系统阈值 <strong>%.2f%%</strong>。</p>
		`, channelId, config.MetricQueueSize, successRate*100, config.MetricSuccessRateThreshold*100),
	)
	_ = notifyRootUser(subject, content)
}

// EnableChannel enable & notify
func EnableChannel(channelId string, channelName string) {
	model.UpdateChannelStatusById(channelId, model.ChannelStatusEnabled)
	logger.SysLog(fmt.Sprintf("channel #%s has been enabled", channelId))
	subject := fmt.Sprintf("渠道状态变更提醒")
	content := message.EmailTemplate(
		subject,
		fmt.Sprintf(`
			<p>您好！</p>
			<p>渠道「<strong>%s</strong>」（#%s）已被重新启用。</p>
			<p>您现在可以继续使用该渠道了。</p>
		`, channelName, channelId),
	)
	_ = notifyRootUser(subject, content)
}

func RecoverMetricDisabledChannel(channelId string, channelName string) {
	model.UpdateChannelStatusById(channelId, model.ChannelStatusEnabled)
	logger.SysLog(fmt.Sprintf("channel #%s has been recovered after metric circuit break", channelId))
	subject := "渠道熔断恢复提醒"
	content := message.EmailTemplate(
		subject,
		fmt.Sprintf(`
			<p>您好！</p>
			<p>渠道「<strong>%s</strong>」（#%s）已从低成功率熔断中自动恢复。</p>
			<p>恢复原因：</p>
			<p style="background-color: #f8f8f8; padding: 10px; border-radius: 4px;">熔断等待时间已结束，渠道已重新进入运行态路由候选，后续真实请求会继续验证该渠道健康状态。</p>
		`, notificationValue(channelName), notificationValue(channelId)),
	)
	_ = notifyRootUser(subject, content)
}

func RecoverMetricDisabledChannelHalfOpen(channelId string, channelName string) {
	model.UpdateChannelStatusById(channelId, model.ChannelStatusHalfOpen)
	logger.SysLog(fmt.Sprintf("channel #%s entered half-open after metric circuit break", channelId))
	subject := "渠道熔断半开探测提醒"
	content := message.EmailTemplate(
		subject,
		fmt.Sprintf(`
			<p>您好！</p>
			<p>渠道「<strong>%s</strong>」（#%s）已进入低成功率熔断半开探测状态。</p>
			<p>恢复策略：</p>
			<p style="background-color: #f8f8f8; padding: 10px; border-radius: 4px;">熔断等待时间已结束，渠道会以低优先级进入运行态候选。下一次探测成功后才完全恢复，失败则重新熔断等待。</p>
		`, notificationValue(channelName), notificationValue(channelId)),
	)
	_ = notifyRootUser(subject, content)
}
