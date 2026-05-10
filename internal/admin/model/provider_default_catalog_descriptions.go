package model

import "strings"

var defaultProviderModelDescriptions = map[string]map[string]string{
	"openai": {
		"gpt-5":              "OpenAI 的通用旗舰文本模型，面向高质量对话、复杂推理、工具调用与长上下文生产任务。",
		"gpt-5.1":            "GPT-5 系列的增量版本，延续旗舰通用能力，适合稳定承载多步骤问答、总结和 Agent 工作流。",
		"gpt-5.1-codex":      "面向代码生成与仓库理解的 Codex 变体，适合补全、重构、调试和 Agent 式开发场景。",
		"gpt-5.1-codex-max":  "GPT-5.1 Codex 的高能力档位，偏向更强的代码推理、复杂修改与长链路开发任务。",
		"gpt-5.1-codex-mini": "GPT-5.1 Codex 的轻量版本，在较低成本下提供代码补全、脚本生成和快速修复能力。",
		"gpt-5.2":            "GPT-5 系列后续升级版本，定位为更强的通用文本与推理模型，适合高质量生产和工具编排。",
		"gpt-5.2-codex":      "GPT-5.2 的代码专用变体，侧重工程实现、代码理解、补丁生成与自动化开发协作。",
		"gpt-5.3-codex":      "更新一代的 Codex 模型，适合更复杂的代码改写、错误定位与多文件开发任务。",
		"gpt-5.4":            "GPT-5 系列中的更高阶旗舰模型，适合高准确率、多工具协同和更复杂的推理型文本任务。",
		"gpt-5.4-mini":       "GPT-5.4 的轻量版本，兼顾速度与成本，适合大量在线问答、分类与轻量 Agent 流程。",
		"gpt-5.5":            "GPT-5 系列的高能力后续版本，面向更重的推理、规划、生成质量和复杂执行场景。",
		"gpt-5-mini":         "GPT-5 系列的低成本文本模型，适合通用对话、简单分析与大规模调用。",
		"gpt-5-nano":         "GPT-5 系列的超轻量档位，重点在最低成本和高吞吐，适合路由、抽取和简单自动化。",
		"gpt-5-pro":          "GPT-5 家族中的高性能档位，面向高难度推理、复杂写作、专业分析和更长执行链路。",
		"gpt-5-codex":        "GPT-5 的代码模型，适合代码生成、仓库理解、修复建议和开发助手场景。",
		"codex-mini-latest":  "OpenAI 当前轻量级 Codex 路线模型，适合快速代码补全、脚本生成和低成本开发辅助。",
		"gpt-4.1":            "OpenAI 的通用文本模型，强调稳定的指令遵循、工具调用和生产级任务处理能力。",
		"gpt-4.1-mini":       "GPT-4.1 的轻量版本，适合成本敏感的聊天、抽取、分类和简单工作流自动化。",
		"gpt-4.1-nano":       "GPT-4.1 的超低成本版本，适合高频短请求、标签生成、过滤和轻量推断。",
		"gpt-4o":             "OpenAI 的原生多模态主力模型，适合文本、视觉和工具混合的通用交互任务。",
		"gpt-4o-mini":        "GPT-4o 的轻量版本，适合更低成本的多模态问答、客服和通用交互接口。",
		"o3":                 "OpenAI 的强化推理模型，适合需要分步分析、计划和高可靠答案的复杂任务。",
		"o4-mini":            "OpenAI 的轻量推理模型，在更快响应与较低成本下提供较强的分析能力。",
		"gpt-image-2":        "OpenAI 的图像生成/编辑模型，支持在 Responses 与 Images 接口中完成图像生成、修改和多模态理解。",
		"gpt-image-1":        "OpenAI 的图像生成模型，支持按质量与尺寸配置输出，适合通用图片生成与创意制作。",
		"dall-e-3":           "OpenAI 的高质量图像生成模型，擅长根据自然语言提示生成更精细、更一致的静态图像。",
		"gpt-realtime":       "OpenAI 的实时多模态语音交互模型，适合低延迟语音对话、实时工具调用和互动式应用。",
		"gpt-realtime-mini":  "实时语音模型的轻量版本，适合对延迟和成本更敏感的语音助手与通话场景。",
		"gpt-audio":          "OpenAI 的音频理解与生成模型，适合语音输入输出、多模态会话与语音驱动工作流。",
		"gpt-audio-mini":     "音频模型的轻量版本，适合大规模语音转文本、语音交互和低成本多模态应用。",
		"whisper-1":          "OpenAI 的语音识别模型，主要用于高质量语音转文本、字幕生成和录音转写。",
		"tts-1":              "OpenAI 的文本转语音模型，适合将文本快速合成为自然语音输出。",
		"tts-1-hd":           "OpenAI 的高质量文本转语音模型，适合对音质和表现力要求更高的语音场景。",
		"sora-2":             "OpenAI 的视频生成模型，面向文生视频和更复杂的视觉内容创作任务。",
	},
	"baidu": {
		"ernie-4.5-turbo-128k":     "百度千帆的长上下文高性能通用模型，适合复杂问答、知识整合和企业级文本生成。",
		"ernie-x1.1-preview":       "百度的推理型预览模型，偏向更强分析、规划和多步骤问题求解。",
		"ernie-4.5-vl-32k-preview": "百度文心 4.5 的视觉语言预览模型，支持图文理解、识别与多模态问答。",
	},
	"anthropic": {
		"claude-opus-4-7":            "Anthropic 的顶级 Claude Opus 模型，面向最复杂的推理、研究、写作和高可靠 Agent 任务。",
		"claude-opus-4-6":            "Claude Opus 4 系列高能力版本，适合复杂分析、专业内容生成和长上下文协作。",
		"claude-opus-4-6-thinking":   "Opus 4.6 的思考型变体，适合需要显式推理过程和更强规划能力的任务。",
		"claude-sonnet-4-6":          "Claude Sonnet 4 系列主力模型，兼顾质量、速度与成本，适合大多数生产应用。",
		"claude-opus-4-5":            "Claude Opus 4.5 版本，定位为高质量推理与写作旗舰模型。",
		"claude-opus-4-5-20251101":   "Claude Opus 4.5 的固定发布日期快照，适合需要版本可复现性的生产环境。",
		"claude-opus-4-1":            "Claude Opus 4.1 旗舰模型，强调复杂推理、长文生成和高质量专业输出。",
		"claude-opus-4-1-20250805":   "Claude Opus 4.1 的固定版本快照，便于锁定模型行为和上线回归。",
		"claude-sonnet-4-5":          "Claude Sonnet 4.5 是 Anthropic 的均衡型主力模型，适合通用对话、编码和工作流自动化。",
		"claude-sonnet-4-5-20250929": "Claude Sonnet 4.5 的固定日期版本，适合需要稳定版本控制的集成场景。",
		"claude-haiku-4-5":           "Claude Haiku 4.5 的高速低成本版本，适合在线问答、分类、抽取和批量处理。",
		"claude-haiku-4-5-20251001":  "Claude Haiku 4.5 的固定版本快照，适合在追求稳定行为时进行大规模调用。",
		"claude-3-5-haiku-20241022":  "Claude 3.5 Haiku 的轻量快照版本，适合实时响应、简单自动化和成本敏感任务。",
	},
	"deepseek": {
		"deepseek-chat":     "DeepSeek 兼容名称中的通用对话模型，对应官方较新的非思考模式主力模型，适合常规文本生成。",
		"deepseek-reasoner": "DeepSeek 兼容名称中的推理模型，对应官方思考模式主力模型，适合复杂分析与多步骤问题求解。",
		"deepseek-v3.1":     "DeepSeek 的通用文本模型版本，面向更强生成质量、上下文处理与多轮对话能力。",
	},
	"google": {
		"gemini-2.5-pro":                    "Google Gemini 2.5 系列高能力模型，适合复杂推理、编码、长上下文分析和多模态工作流。",
		"gemini-2.5-flash":                  "Gemini 2.5 Flash 是 Google 的高性价比主力模型，兼顾速度、工具调用和多场景文本任务。",
		"gemini-2.5-flash-lite":             "Gemini 2.5 Flash Lite 是更轻量的低成本版本，适合高频问答、分类和批量处理。",
		"gemini-live-2.5-flash-preview":     "Google 的实时语音/多模态预览模型，适合低延迟语音交互、实时助手与流式应用。",
		"gemini-2.5-flash-image-preview":    "Gemini 2.5 Flash 的图像预览能力版本，适合图像理解、视觉问答与多模态生成实验。",
		"imagen-4.0-generate-preview-06-06": "Google Imagen 4 的图像生成预览模型，面向高质量文生图和创意图像生产。",
		"veo-3.0-generate-preview":          "Google Veo 3 的视频生成预览模型，适合文本生成视频和更复杂的镜头化内容创作。",
	},
	"hunyuan": {
		"hunyuan-t1":            "腾讯混元的推理型文本模型，适合复杂分析、数学逻辑和多步骤问答任务。",
		"hunyuan-t1-latest":     "腾讯混元 T1 的 latest 别名，便于持续跟踪官方当前推荐的推理模型版本。",
		"hunyuan-turbos":        "腾讯混元 TurboS 是偏速度和成本效率的通用文本模型，适合高并发在线服务。",
		"hunyuan-turbos-latest": "腾讯混元 TurboS 的 latest 别名，用于稳定接入官方当前主力快速模型。",
		"hunyuan-image":         "腾讯混元图像模型，适合通用文生图、创意海报与视觉内容生成。",
		"hunyuan-video":         "腾讯混元视频模型，适合文生视频和动态视觉内容生成。",
	},
	"minimax": {
		"minimax-m2.1":           "MiniMax M2.1 的通用文本模型，适合对话、内容生成和多轮交互场景。",
		"minimax-m2.1-highspeed": "MiniMax M2.1 的高速版本，面向更低延迟响应和在线交互型任务。",
		"minimax-m2":             "MiniMax M2 系列通用模型，适合稳定的文本生成、问答和业务流程嵌入。",
		"speech-2.5-hd-preview":  "MiniMax 的高质量语音预览模型，适合文本转语音或更自然的语音交互输出。",
		"image-01":               "MiniMax 的图像生成模型，适合通用图片创作、营销素材和视觉内容生产。",
		"video-01":               "MiniMax 的视频生成模型，适合短视频生成、创意演示和动态视觉表达。",
	},
	"xai": {
		"grok-4":                      "xAI 的 Grok 4 旗舰模型，定位为高能力通用推理与知识问答模型。",
		"grok-4-fast-non-reasoning":   "Grok 4 Fast 的非推理版本，面向更快响应速度的通用文本生成和问答。",
		"grok-4-fast-reasoning":       "Grok 4 Fast 的推理版本，在较低延迟下提供更强分析和多步骤求解能力。",
		"grok-4-1-fast-non-reasoning": "Grok 4.1 Fast 的非推理版本，适合高并发在线问答和低延迟文本场景。",
		"grok-4-1-fast-reasoning":     "Grok 4.1 Fast 的推理版本，适合更复杂但仍需快速返回的分析任务。",
		"grok-code-fast-1":            "xAI 的代码模型，侧重代码生成、修复、解释和开发助手能力。",
		"grok-2-image-1212":           "xAI 的图像生成模型，适合基于自然语言进行图像创建和创意视觉输出。",
	},
	"qwen": {
		"qwen-max-latest":        "阿里云百炼的旗舰通义千问文本模型，适合复杂问答、长文生成和企业级智能体任务。",
		"qwen-plus-latest":       "通义千问的均衡型主力模型，兼顾成本、质量与通用业务可用性。",
		"qwen-turbo-latest":      "通义千问的高速低成本模型，适合高频在线问答、分类和批量内容处理。",
		"qwen-vl-max-latest":     "通义千问视觉语言旗舰模型，适合图文理解、文档识别与视觉问答。",
		"qvq-max-latest":         "Qwen 的视觉推理模型，强调更强的图像理解、视觉分析和多模态推理。",
		"qwen-tts-latest":        "通义千问的文本转语音模型，适合将文本内容合成为自然语音。",
		"qwen-omni-turbo-latest": "通义千问的全模态轻量模型，适合文本、语音和视觉混合交互场景。",
	},
	"stepfun": {
		"step-3.5-flash":       "阶跃星辰的高速文本模型，适合低延迟问答、内容生成和在线服务。",
		"step-2-mini":          "阶跃星辰的轻量文本模型，适合成本敏感的通用问答与批处理任务。",
		"step-1o-turbo-vision": "阶跃的视觉理解模型，适合图文问答、图片分析和多模态输入场景。",
		"step-1o-audio":        "阶跃的音频模型，适合语音理解、语音交互和多模态音频工作流。",
		"step-1x-medium":       "阶跃的图像生成模型，适合通用图片创作、海报和视觉内容生成。",
	},
	"mistral": {
		"mistral-large-latest":  "Mistral 的旗舰文本模型，适合复杂推理、企业知识助手、代码和长上下文处理。",
		"mistral-medium-latest": "Mistral 的均衡型文本模型，适合通用生产任务并兼顾成本与速度。",
		"pixtral-large-latest":  "Mistral 的视觉语言模型，适合图文理解、文档分析和多模态问答。",
		"voxtral-mini-latest":   "Mistral 的语音/音频模型，适合语音理解、语音助手和音频驱动工作流。",
	},
	"cohere": {
		"command-a-03-2025":   "Cohere 的 Command A 系列模型，适合企业检索增强、工具调用和通用文本任务。",
		"command-r7b-12-2024": "Cohere 的轻量检索增强模型，适合成本敏感的 RAG、问答和业务助理场景。",
	},
	"zhipu": {
		"glm-4.5-air":      "智谱 GLM-4.5 Air 文本模型，适合通用对话、推理和成本敏感的业务集成。",
		"glm-4v-plus-0111": "智谱的视觉语言模型，适合图像理解、图文问答和视觉输入处理。",
		"glm-4-voice":      "智谱的语音模型，适合语音理解、语音对话和音频交互应用。",
		"cogview-4-250304": "智谱的 CogView 图像生成模型，适合高质量文生图和创意图片生产。",
		"cogvideox-flash":  "智谱的 CogVideoX 轻量视频模型，适合更快速的视频生成与动态内容创作。",
	},
}

func defaultProviderModelDescription(provider string, modelName string) string {
	normalizedProvider := strings.TrimSpace(strings.ToLower(provider))
	if normalizedProvider == "" {
		return ""
	}
	canonicalModel := canonicalizeModelNameForProvider(normalizedProvider, modelName)
	canonicalModel = strings.TrimSpace(strings.ToLower(canonicalModel))
	if canonicalModel == "" {
		return ""
	}
	providerDescriptions, ok := defaultProviderModelDescriptions[normalizedProvider]
	if !ok {
		return ""
	}
	return strings.TrimSpace(providerDescriptions[canonicalModel])
}
