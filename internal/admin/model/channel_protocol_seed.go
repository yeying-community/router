package model

import (
	"strings"

	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/logger"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"gorm.io/gorm"
)

type channelProtocolSeed struct {
	ID          int
	Label       string
	Color       string
	Description string
	Tip         string
}

var defaultChannelProtocolSeeds = []channelProtocolSeed{
	{ID: 1, Label: "OpenAI", Color: "green"},
	{ID: 14, Label: "Anthropic", Color: "black"},
	{ID: 33, Label: "AWS", Color: "black"},
	{ID: 3, Label: "Azure", Color: "olive"},
	{ID: 11, Label: "PaLM2", Color: "orange"},
	{ID: 24, Label: "Gemini", Color: "orange"},
	{ID: 51, Label: "Gemini (OpenAI)", Color: "orange", Description: "Gemini OpenAI 兼容格式"},
	{ID: 28, Label: "Mistral AI", Color: "orange"},
	{ID: 41, Label: "Novita", Color: "purple"},
	{ID: 40, Label: "字节火山引擎", Color: "blue", Description: "原字节跳动豆包"},
	{ID: 15, Label: "百度文心千帆", Color: "blue", Tip: "请前往<a href=\"https://console.bce.baidu.com/qianfan/ais/console/applicationConsole/application/v1\" target=\"_blank\">此处</a>获取 AK（API Key）以及 SK（Secret Key），注意，V2 版本接口请使用 <strong>百度文心千帆 V2 </strong>渠道类型"},
	{ID: 47, Label: "百度文心千帆 V2", Color: "blue", Tip: "请前往<a href=\"https://console.bce.baidu.com/iam/#/iam/apikey/list\" target=\"_blank\">此处</a>获取 API Key，注意本渠道仅支持<a target=\"_blank\" href=\"https://cloud.baidu.com/doc/WENXINWORKSHOP/s/em4tsqo3v\">推理服务 V2</a>相关模型"},
	{ID: 17, Label: "阿里通义千问", Color: "orange", Tip: "阿里兼容模式与原生接口统一使用该渠道"},
	{ID: 18, Label: "讯飞星火认知", Color: "blue", Tip: "本渠道基于讯飞 WebSocket 版本 API，如需 HTTP 版本，请使用<strong>讯飞星火认知 V2</strong>渠道"},
	{ID: 48, Label: "讯飞星火认知 V2", Color: "blue", Tip: "HTTP 版本的讯飞接口，前往<a href=\"https://console.xfyun.cn/services/cbm\" target=\"_blank\">此处</a>获取 HTTP 服务接口认证密钥"},
	{ID: 16, Label: "智谱 ChatGLM", Color: "violet"},
	{ID: 19, Label: "360 智脑", Color: "blue"},
	{ID: 25, Label: "Moonshot AI", Color: "black"},
	{ID: 23, Label: "腾讯混元", Color: "teal"},
	{ID: 26, Label: "百川大模型", Color: "orange"},
	{ID: 27, Label: "MiniMax", Color: "red"},
	{ID: 29, Label: "Groq", Color: "orange"},
	{ID: 30, Label: "Ollama", Color: "black"},
	{ID: 31, Label: "零一万物", Color: "green"},
	{ID: 32, Label: "阶跃星辰", Color: "blue"},
	{ID: 34, Label: "Coze", Color: "blue"},
	{ID: 35, Label: "Cohere", Color: "blue"},
	{ID: 36, Label: "DeepSeek", Color: "black"},
	{ID: 37, Label: "Cloudflare", Color: "orange"},
	{ID: 38, Label: "DeepL", Color: "black"},
	{ID: 39, Label: "together.ai", Color: "blue"},
	{ID: 42, Label: "VertexAI", Color: "blue"},
	{ID: 43, Label: "Proxy", Color: "blue"},
	{ID: 44, Label: "SiliconFlow", Color: "blue"},
	{ID: 45, Label: "xAI", Color: "blue"},
	{ID: 46, Label: "Replicate", Color: "blue"},
	{ID: 8, Label: "自定义渠道", Color: "pink", Description: "不推荐使用，请使用 OpenAI 渠道类型", Tip: "不推荐使用，请使用 <strong>OpenAI</strong>渠道类型。注意，这里所需要填入的代理地址仅会在实际请求时替换域名部分，如果你想填入 OpenAI SDK 中所要求的 Base URL，请使用 OpenAI 渠道类型"},
	{ID: 22, Label: "知识库：FastGPT", Color: "blue"},
	{ID: 21, Label: "知识库：AI Proxy", Color: "purple"},
	{ID: 20, Label: "OpenRouter", Color: "black"},
	{ID: 2, Label: "代理：API2D", Color: "blue"},
	{ID: 5, Label: "代理：OpenAI-SB", Color: "brown"},
	{ID: 7, Label: "代理：OhMyGPT", Color: "purple"},
	{ID: 10, Label: "代理：AI Proxy", Color: "purple"},
	{ID: 4, Label: "代理：CloseAI", Color: "teal"},
	{ID: 6, Label: "代理：OpenAI Max", Color: "violet"},
	{ID: 9, Label: "代理：AI.LS", Color: "yellow"},
	{ID: 12, Label: "代理：API2GPT", Color: "blue"},
	{ID: 13, Label: "代理：AIGC2D", Color: "purple"},
}

func ensureChannelProtocolCatalogSeededWithDB(db *gorm.DB) error {
	if err := db.AutoMigrate(&ChannelProtocolCatalog{}); err != nil {
		return err
	}
	var count int64
	if err := db.Model(&ChannelProtocolCatalog{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	defaults := buildDefaultChannelProtocolCatalog(helper.GetTimestamp())
	if len(defaults) == 0 {
		return nil
	}
	if err := db.Create(&defaults).Error; err != nil {
		return err
	}
	logger.SysLogf("migration: initialized channel protocol catalog with %d default items", len(defaults))
	return nil
}

func resolveChannelProtocolNameByProtocolID(id int) string {
	if id >= 0 && id < len(relaychannel.ChannelProtocolNames) {
		return strings.TrimSpace(relaychannel.ProtocolByType(id))
	}
	return ""
}

func buildDefaultChannelProtocolCatalog(now int64) []ChannelProtocolCatalog {
	items := make([]ChannelProtocolCatalog, 0, len(defaultChannelProtocolSeeds))
	for idx, seed := range defaultChannelProtocolSeeds {
		name := resolveChannelProtocolNameByProtocolID(seed.ID)
		if name == "" {
			continue
		}
		items = append(items, ChannelProtocolCatalog{
			Name:        name,
			ProtocolID:  seed.ID,
			Label:       strings.TrimSpace(seed.Label),
			Color:       strings.TrimSpace(seed.Color),
			Description: strings.TrimSpace(seed.Description),
			Tip:         strings.TrimSpace(seed.Tip),
			Source:      "default",
			Enabled:     true,
			SortOrder:   idx + 1,
			UpdatedAt:   now,
		})
	}
	return items
}
