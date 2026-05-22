package openai

import (
	"github.com/yeying-community/router/internal/relay/adaptor/ai360"
	"github.com/yeying-community/router/internal/relay/adaptor/baichuan"
	"github.com/yeying-community/router/internal/relay/adaptor/baiduv2"
	"github.com/yeying-community/router/internal/relay/adaptor/deepseek"
	"github.com/yeying-community/router/internal/relay/adaptor/doubao"
	"github.com/yeying-community/router/internal/relay/adaptor/geminiv2"
	"github.com/yeying-community/router/internal/relay/adaptor/groq"
	"github.com/yeying-community/router/internal/relay/adaptor/lingyiwanwu"
	"github.com/yeying-community/router/internal/relay/adaptor/minimax"
	"github.com/yeying-community/router/internal/relay/adaptor/mistral"
	"github.com/yeying-community/router/internal/relay/adaptor/moonshot"
	"github.com/yeying-community/router/internal/relay/adaptor/novita"
	"github.com/yeying-community/router/internal/relay/adaptor/openrouter"
	"github.com/yeying-community/router/internal/relay/adaptor/siliconflow"
	"github.com/yeying-community/router/internal/relay/adaptor/stepfun"
	"github.com/yeying-community/router/internal/relay/adaptor/togetherai"
	"github.com/yeying-community/router/internal/relay/adaptor/xai"
	"github.com/yeying-community/router/internal/relay/adaptor/xunfeiv2"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
)

var CompatibleChannels = []int{
	relaychannel.Azure,
	relaychannel.AI360,
	relaychannel.Moonshot,
	relaychannel.Baichuan,
	relaychannel.Minimax,
	relaychannel.Doubao,
	relaychannel.Mistral,
	relaychannel.Groq,
	relaychannel.LingYiWanWu,
	relaychannel.StepFun,
	relaychannel.DeepSeek,
	relaychannel.TogetherAI,
	relaychannel.Novita,
	relaychannel.SiliconFlow,
	relaychannel.XAI,
	relaychannel.BaiduV2,
	relaychannel.XunfeiV2,
}

func GetCompatibleChannelMeta(channelProtocol int) (string, []string) {
	switch channelProtocol {
	case relaychannel.Azure:
		return "azure", ModelList
	case relaychannel.AI360:
		return "360", ai360.ModelList
	case relaychannel.Moonshot:
		return "moonshot", moonshot.ModelList
	case relaychannel.Baichuan:
		return "baichuan", baichuan.ModelList
	case relaychannel.Minimax:
		return "minimax", minimax.ModelList
	case relaychannel.Mistral:
		return "mistralai", mistral.ModelList
	case relaychannel.Groq:
		return "groq", groq.ModelList
	case relaychannel.LingYiWanWu:
		return "lingyiwanwu", lingyiwanwu.ModelList
	case relaychannel.StepFun:
		return "stepfun", stepfun.ModelList
	case relaychannel.DeepSeek:
		return "deepseek", deepseek.ModelList
	case relaychannel.TogetherAI:
		return "together.ai", togetherai.ModelList
	case relaychannel.Doubao:
		return "doubao", doubao.ModelList
	case relaychannel.Novita:
		return "novita", novita.ModelList
	case relaychannel.SiliconFlow:
		return "siliconflow", siliconflow.ModelList
	case relaychannel.XAI:
		return "xai", xai.ModelList
	case relaychannel.BaiduV2:
		return "baiduv2", baiduv2.ModelList
	case relaychannel.XunfeiV2:
		return "xunfeiv2", xunfeiv2.ModelList
	case relaychannel.OpenRouter:
		return "openrouter", openrouter.ModelList
	case relaychannel.GeminiOpenAICompatible:
		return "geminiv2", geminiv2.ModelList
	default:
		return "openai", ModelList
	}
}
