package monitor

import (
	"fmt"
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

// DisableChannel disable & notify
func DisableChannel(channelId string, channelName string, reason string) {
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
