package model

import "strings"

func defaultProviderModelDescription(provider string, modelName string, modelType string) string {
	provider = strings.TrimSpace(strings.ToLower(provider))
	modelName = strings.TrimSpace(strings.ToLower(modelName))
	switch provider {
	case "openai":
		return openAIProviderModelDescription(modelName)
	case "anthropic":
		return anthropicProviderModelDescription(modelName)
	case "google":
		return googleProviderModelDescription(modelName)
	case "deepseek":
		return deepSeekProviderModelDescription(modelName)
	case "xai":
		return xAIProviderModelDescription(modelName)
	case "qwen":
		return qwenProviderModelDescription(modelName)
	case "mistral":
		return mistralProviderModelDescription(modelName)
	case "cohere":
		return cohereProviderModelDescription(modelName)
	case "baidu":
		return baiduProviderModelDescription(modelName)
	case "hunyuan":
		return hunyuanProviderModelDescription(modelName)
	case "minimax":
		return minimaxProviderModelDescription(modelName)
	case "stepfun":
		return stepFunProviderModelDescription(modelName)
	case "zhipu":
		return zhipuProviderModelDescription(modelName)
	case "volcengine":
		return volcengineProviderModelDescription(modelName)
	default:
		return ""
	}
}

func defaultProviderModelDeleted(provider string, modelName string) bool {
	provider = strings.TrimSpace(strings.ToLower(provider))
	modelName = strings.TrimSpace(strings.ToLower(modelName))
	switch provider {
	case "anthropic":
		return modelName == "claude-3-5-haiku-20241022"
	case "google":
		return modelName == "gemini-live-2.5-flash-preview"
	case "xai":
		return modelName == "grok-2-image-1212"
	default:
		return false
	}
}

func defaultProviderModelStatus(provider string, modelName string) string {
	provider = strings.TrimSpace(strings.ToLower(provider))
	modelName = strings.TrimSpace(strings.ToLower(modelName))
	switch provider {
	case "openai":
		if modelName == "codex-mini-latest" {
			return ProviderModelStatusDeprecated
		}
	case "anthropic":
		if modelName == "claude-3-5-haiku-20241022" {
			return ProviderModelStatusDeprecated
		}
	case "google":
		if modelName == "gemini-live-2.5-flash-preview" {
			return ProviderModelStatusDeprecated
		}
	case "xai":
		if modelName == "grok-2-image-1212" {
			return ProviderModelStatusDeprecated
		}
	}
	return ProviderModelStatusActive
}

func openAIProviderModelDescription(modelName string) string {
	switch modelName {
	case "gpt-5":
		return "GPT-5 是 OpenAI 面向复杂问题求解、编码和代理任务的旗舰模型。"
	case "gpt-5.1":
		return "GPT-5.1 是 GPT-5 系列中的通用高能力模型，适合复杂推理和生产任务。"
	case "gpt-5.1-codex":
		return "GPT-5.1 Codex 是面向代码生成、修改和工程代理任务优化的模型。"
	case "gpt-5.1-codex-max":
		return "GPT-5.1 Codex Max 是更偏高能力的 Codex 版本，适合复杂代码工作流。"
	case "gpt-5.1-codex-mini":
		return "GPT-5.1 Codex Mini 是更轻量的代码模型，适合成本敏感的工程任务。"
	case "gpt-5.2":
		return "GPT-5.2 是 GPT-5 系列中的高能力通用模型，适合复杂推理和多步骤任务。"
	case "gpt-5.2-codex":
		return "GPT-5.2 Codex 是 GPT-5.2 的代码版本，适合代码理解、生成和修复。"
	case "gpt-5.3-codex":
		return "GPT-5.3 Codex 是面向编程代理、代码生成与重构的 Codex 模型。"
	case "gpt-5.4":
		return "GPT-5.4 是 GPT-5 系列中的高能力模型，适合复杂推理和高质量生成。"
	case "gpt-5.4-mini":
		return "GPT-5.4 Mini 是 GPT-5.4 的轻量版本，兼顾速度与成本。"
	case "gpt-5.4-nano":
		return "GPT-5.4 Nano 是 GPT-5.4 系列中的超轻量版本，适合高并发简单任务。"
	case "gpt-5.4-pro":
		return "GPT-5.4 Pro 是面向高难度专业任务的 GPT-5.4 版本。"
	case "gpt-5.5":
		return "GPT-5.5 是 OpenAI 当前更高能力的通用模型，适合复杂推理和生产场景。"
	case "gpt-5.5-pro":
		return "GPT-5.5 Pro 是 GPT-5.5 的高能力版本，适合高难度专业任务。"
	case "gpt-5-mini":
		return "GPT-5 Mini 是 GPT-5 的小型版本，适合日常对话、摘要和批量处理。"
	case "gpt-5-nano":
		return "GPT-5 Nano 是 GPT-5 的超轻量版本，适合简单分类、抽取和高吞吐任务。"
	case "gpt-5-pro":
		return "GPT-5 Pro 是 GPT-5 的高算力版本，适合更复杂、更高精度的推理任务。"
	case "gpt-5-codex":
		return "GPT-5 Codex 是 GPT-5 系列中面向代码理解、生成和代理执行的模型。"
	case "codex-mini-latest":
		return "Codex Mini 是 OpenAI 的轻量代码模型，当前已标记为 deprecated。"
	case "gpt-4.1":
		return "GPT-4.1 是 OpenAI 的通用模型，适合复杂指令遵循与生产级文本任务。"
	case "gpt-4.1-mini":
		return "GPT-4.1 Mini 是 GPT-4.1 的轻量版本，适合延迟和成本更敏感的场景。"
	case "gpt-4.1-nano":
		return "GPT-4.1 Nano 是 GPT-4.1 的超轻量版本，适合简单高并发任务。"
	case "gpt-4o":
		return "GPT-4o 是 OpenAI 的原生多模态模型，适合文本、图像和语音交互。"
	case "gpt-4o-mini":
		return "GPT-4o Mini 是低成本多模态模型，适合大规模常规任务。"
	case "o3":
		return "o3 是 OpenAI 的推理模型，适合数学、代码和多步骤问题求解。"
	case "o4-mini":
		return "o4-mini 是轻量推理模型，适合兼顾速度和推理质量的任务。"
	case "gpt-image-2":
		return "GPT Image 2 是 OpenAI 的图像生成模型，适合高质量图片生成。"
	case "gpt-image-1.5":
		return "GPT Image 1.5 是更具性价比的图像生成模型，适合图像生成与编辑。"
	case "gpt-image-1":
		return "GPT Image 1 是 OpenAI 的图像生成与编辑模型，适合多种图片工作流。"
	case "gpt-image-1-mini":
		return "GPT Image 1 Mini 是更低成本的图像生成模型，适合成本敏感场景。"
	case "dall-e-3":
		return "DALL·E 3 是 OpenAI 的图像生成模型，适合根据提示词生成高质量图像。"
	case "gpt-realtime":
		return "GPT Realtime 是 OpenAI 的实时多模态会话模型，适合低延迟语音与文本交互。"
	case "gpt-realtime-2":
		return "GPT Realtime 2 是更高能力的实时多模态模型，适合实时语音与文本对话。"
	case "gpt-realtime-1.5":
		return "GPT Realtime 1.5 是较轻量的实时模型，适合延迟敏感的会话场景。"
	case "gpt-realtime-mini":
		return "GPT Realtime Mini 是低成本实时模型，适合高并发语音和文本交互。"
	case "gpt-realtime-translate":
		return "GPT Realtime Translate 是实时翻译模型，适合低延迟语音翻译场景。"
	case "gpt-4o-mini-tts":
		return "GPT-4o Mini TTS 是文本转语音模型，适合以较低成本生成自然语音。"
	case "gpt-audio":
		return "GPT Audio 是支持音频输入输出的模型，可用于 Chat Completions 音频场景。"
	case "gpt-audio-mini":
		return "GPT Audio Mini 是更低成本的 GPT Audio 版本，支持音频输入输出。"
	case "whisper-1":
		return "Whisper-1 是 OpenAI 的语音识别模型，适合转写和翻译。"
	case "tts-1":
		return "TTS-1 是 OpenAI 的文本转语音模型，适合快速语音生成。"
	case "tts-1-hd":
		return "TTS-1-HD 是更高音质的文本转语音模型。"
	case "sora-2":
		return "Sora 2 是 OpenAI 的视频生成模型，适合根据文本或多模态输入生成视频。"
	case "sora-2-pro":
		return "Sora 2 Pro 是更高能力的视频生成模型，适合更复杂的视频生成任务。"
	default:
		return ""
	}
}

func anthropicProviderModelDescription(modelName string) string {
	switch modelName {
	case "claude-opus-4-7":
		return "Claude Opus 4.7 是 Anthropic 的高能力 Claude 模型，适合复杂分析与代码任务。"
	case "claude-opus-4-6":
		return "Claude Opus 4.6 是 Anthropic 的高能力 Claude 模型，适合复杂任务与长上下文分析。"
	case "claude-opus-4-6-thinking":
		return "Claude Opus 4.6 Thinking 是带更强推理特性的 Claude Opus 版本。"
	case "claude-sonnet-4-6":
		return "Claude Sonnet 4.6 是均衡型 Claude 模型，适合大多数生产场景。"
	case "claude-opus-4-5":
		return "Claude Opus 4.5 是 Anthropic 的高能力 Claude 模型，适合复杂分析与代码任务。"
	case "claude-opus-4-5-20251101":
		return "Claude Opus 4.5 是 Anthropic 的高能力 Claude 模型，适合复杂分析与代码任务。"
	case "claude-opus-4-1":
		return "Claude Opus 4.1 是 Anthropic 的高能力模型，适合高复杂度分析和代码任务。"
	case "claude-opus-4-1-20250805":
		return "Claude Opus 4.1 是 Anthropic 的高能力模型，适合高复杂度分析和代码任务。"
	case "claude-sonnet-4-5":
		return "Claude Sonnet 4.5 是均衡型 Claude 模型，适合大多数生产任务。"
	case "claude-sonnet-4-5-20250929":
		return "Claude Sonnet 4.5 是均衡型 Claude 模型，适合大多数生产任务。"
	case "claude-haiku-4-5":
		return "Claude Haiku 4.5 是轻量快速 Claude 模型，适合低延迟与高吞吐任务。"
	case "claude-haiku-4-5-20251001":
		return "Claude Haiku 4.5 是轻量快速 Claude 模型，适合低延迟与高吞吐任务。"
	case "claude-3-5-haiku-20241022":
		return ""
	default:
		return ""
	}
}

func googleProviderModelDescription(modelName string) string {
	switch modelName {
	case "gemini-2.5-pro":
		return "Gemini 2.5 Pro 是 Google 的高能力多模态模型，适合复杂推理和长上下文任务。"
	case "gemini-2.5-flash":
		return "Gemini 2.5 Flash 是低延迟多模态模型，适合高性价比交互场景。"
	case "gemini-2.5-flash-lite":
		return "Gemini 2.5 Flash-Lite 是更轻量的 Flash 版本，适合低成本高并发任务。"
	case "gemini-live-2.5-flash-preview":
		return ""
	case "gemini-2.5-flash-image-preview":
		return "Gemini 2.5 Flash Image Preview 是支持图像生成与理解的多模态模型。"
	case "imagen-4.0-generate-preview-06-06":
		return "Imagen 4 是 Google 的图像生成模型，适合高质量图片生成。"
	case "veo-3.0-generate-preview":
		return "Veo 3 是 Google 的视频生成模型，适合根据文本生成视频。"
	default:
		return ""
	}
}

func deepSeekProviderModelDescription(modelName string) string {
	switch modelName {
	case "deepseek-chat":
		return "DeepSeek Chat 是 DeepSeek 的通用对话模型别名，适合日常生成与编码任务。"
	case "deepseek-reasoner":
		return "DeepSeek Reasoner 是 DeepSeek 的推理模型别名，适合数学、代码和复杂问题求解。"
	case "deepseek-v3.1":
		return "DeepSeek V3.1 是 DeepSeek 的通用模型，适合对话、生成与编程任务。"
	default:
		return ""
	}
}

func xAIProviderModelDescription(modelName string) string {
	switch modelName {
	case "grok-4.20":
		return "Grok 4.20 是 xAI 当前更新的旗舰推理模型。"
	case "grok-4.3":
		return "Grok 4.3 是 xAI 的高能力通用推理模型。"
	case "grok-4":
		return "Grok 4 是 xAI 的通用推理模型，适合复杂分析和生成任务。"
	case "grok-4-fast-non-reasoning":
		return "Grok 4 Fast Non-Reasoning 是更低延迟的 Grok 4 版本，适合快速响应任务。"
	case "grok-4-fast-reasoning":
		return "Grok 4 Fast Reasoning 是更低延迟的 Grok 4 推理版本。"
	case "grok-4-1-fast-non-reasoning":
		return "Grok 4.1 Fast Non-Reasoning 是低延迟通用模型，官方已标注退役计划。"
	case "grok-4-1-fast-reasoning":
		return "Grok 4.1 Fast Reasoning 是低延迟推理模型，官方已标注退役计划。"
	case "grok-code-fast-1":
		return "Grok Code Fast 1 是面向代码理解、生成和修复的模型。"
	case "grok-2-image-1212":
		return ""
	default:
		return ""
	}
}

func volcengineProviderModelDescription(modelName string) string {
	switch modelName {
	case "doubao-seed-1.6":
		return "Doubao Seed 1.6 是火山方舟的通用高能力模型，适合复杂对话、推理和多模态任务。"
	case "doubao-seed-1.6-thinking":
		return "Doubao Seed 1.6 Thinking 是火山方舟的强制思考模型，适合更强调推理过程的复杂任务。"
	case "doubao-seed-1.6-flash":
		return "Doubao Seed 1.6 Flash 是更低延迟、更低成本的轻量版本，适合高并发常规交互。"
	case "seed1.6-embedding":
		return "Seed1.6-Embedding 是火山方舟的全模态向量化模型，支持文本、图像和视频混合模态检索。"
	case "doubao-seed-code-preview-latest":
		return "Doubao Seed Code 是火山方舟面向代码生成、补全和工程任务优化的编程模型。"
	default:
		return ""
	}
}

func qwenProviderModelDescription(modelName string) string {
	switch modelName {
	case "qwen3.7-max":
		return "Qwen3.7-Max 是 Qwen 当前高能力通用模型，适合复杂推理、代码和多轮任务。"
	case "qwen3.6-plus":
		return "Qwen3.6-Plus 是均衡型通用模型，适合多数生产场景。"
	case "qwen3.6-flash":
		return "Qwen3.6-Flash 是低延迟低成本通用模型。"
	case "qwen3.5-plus":
		return "Qwen3.5-Plus 是上一代均衡型通用模型。"
	case "qwen3.5-flash":
		return "Qwen3.5-Flash 是上一代低成本通用模型。"
	case "qwen3-max":
		return "Qwen3-Max 是高能力通用模型，适合复杂任务。"
	case "qwen-image-2.0":
		return "Qwen-Image 2.0 是图像生成模型。"
	case "qwen-image-2.0-pro":
		return "Qwen-Image 2.0 Pro 是更高质量的图像生成模型。"
	default:
		return ""
	}
}

func mistralProviderModelDescription(modelName string) string {
	switch modelName {
	case "mistral-large-latest":
		return "Mistral Large 是 Mistral 的高能力通用模型，适合复杂推理和文本生成。"
	case "mistral-medium-latest":
		return "Mistral Medium 是均衡型通用模型，适合多数生产场景。"
	case "pixtral-large-latest":
		return "Pixtral Large 是 Mistral 的视觉语言模型，适合图文理解和多模态任务。"
	case "voxtral-mini-latest":
		return "Voxtral Mini 是 Mistral 的语音模型，适合音频理解与交互。"
	default:
		return ""
	}
}

func cohereProviderModelDescription(modelName string) string {
	switch modelName {
	case "command-a-03-2025":
		return "Command A 是 Cohere 面向企业代理、工具调用和检索增强场景的模型。"
	case "command-r7b-12-2024":
		return "Command R7B 是更轻量的检索增强与工具调用模型。"
	default:
		return ""
	}
}

func baiduProviderModelDescription(modelName string) string {
	switch modelName {
	case "ernie-4.5-turbo-128k":
		return "ERNIE 4.5 Turbo 128K 是百度千帆的长上下文通用模型。"
	case "ernie-x1.1-preview":
		return "ERNIE X1.1 Preview 是百度千帆的推理模型预览版。"
	case "ernie-4.5-vl-32k-preview":
		return "ERNIE 4.5 VL 32K Preview 是百度千帆的视觉语言模型。"
	default:
		return ""
	}
}

func hunyuanProviderModelDescription(modelName string) string {
	switch modelName {
	case "hunyuan-t1", "hunyuan-t1-latest":
		return "Hunyuan T1 是腾讯混元的通用推理模型。"
	case "hunyuan-turbos", "hunyuan-turbos-latest":
		return "Hunyuan TurboS 是腾讯混元的高速通用模型。"
	case "hunyuan-image":
		return "Hunyuan Image 是腾讯混元的图像生成模型。"
	case "hunyuan-video":
		return "Hunyuan Video 是腾讯混元的视频生成模型。"
	default:
		return ""
	}
}

func minimaxProviderModelDescription(modelName string) string {
	switch modelName {
	case "minimax-m2.1":
		return "MiniMax M2.1 是 MiniMax 的通用模型。"
	case "minimax-m2.1-highspeed":
		return "MiniMax M2.1 Highspeed 是更快的通用模型版本。"
	case "minimax-m2":
		return "MiniMax M2 是 MiniMax 的通用模型。"
	case "speech-2.5-hd-preview":
		return "Speech 2.5 HD Preview 是 MiniMax 的高音质语音生成模型。"
	case "image-01":
		return "Image-01 是 MiniMax 的图像生成模型。"
	case "video-01":
		return "Video-01 是 MiniMax 的视频生成模型。"
	default:
		return ""
	}
}

func stepFunProviderModelDescription(modelName string) string {
	switch modelName {
	case "step-3.5-flash":
		return "Step-3.5-Flash 是阶跃星辰的快速通用模型。"
	case "step-2-mini":
		return "Step-2-Mini 是轻量通用模型，适合成本敏感场景。"
	case "step-1o-turbo-vision":
		return "Step-1o-Turbo-Vision 是多模态视觉模型。"
	case "step-1o-audio":
		return "Step-1o-Audio 是语音与音频交互模型。"
	case "step-1x-medium":
		return "Step-1x-Medium 是图像生成模型。"
	default:
		return ""
	}
}

func zhipuProviderModelDescription(modelName string) string {
	switch modelName {
	case "glm-4.5-air":
		return "GLM-4.5-Air 是智谱的通用模型，适合对话和生成任务。"
	case "glm-4v-plus-0111":
		return "GLM-4V-Plus 是智谱的视觉语言模型。"
	case "glm-4-voice":
		return "GLM-4-Voice 是智谱的语音模型。"
	case "cogview-4-250304":
		return "CogView 4 是智谱的图像生成模型。"
	case "cogvideox-flash":
		return "CogVideoX Flash 是智谱的视频生成模型。"
	default:
		return ""
	}
}
