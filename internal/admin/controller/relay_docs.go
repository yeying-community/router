package controller

import "github.com/gin-gonic/gin"

// RelayChatCompletionsDoc godoc
// @Summary Chat completions (OpenAI compatible)
// @Tags public
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.OpenAIChatCompletionsRequest true "Chat completions request"
// @Success 200 {object} docs.OpenAIChatCompletionsResponse
// @Failure 400 {object} docs.OpenAIErrorResponse
// @Failure 401 {object} docs.OpenAIErrorResponse
// @Router /api/v1/public/chat/completions [post]
func RelayChatCompletionsDoc(c *gin.Context) {}

// RelayCompletionsDoc godoc
// @Summary Completions (OpenAI compatible)
// @Tags public
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.OpenAICompletionsRequest true "Completions request"
// @Success 200 {object} docs.OpenAICompletionsResponse
// @Failure 400 {object} docs.OpenAIErrorResponse
// @Failure 401 {object} docs.OpenAIErrorResponse
// @Router /api/v1/public/completions [post]
func RelayCompletionsDoc(c *gin.Context) {}

// RelayEditsDoc godoc
// @Summary Edits (OpenAI compatible)
// @Tags public
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.OpenAIEditRequest true "Edits request"
// @Success 200 {object} docs.OpenAIEditResponse
// @Failure 400 {object} docs.OpenAIErrorResponse
// @Failure 401 {object} docs.OpenAIErrorResponse
// @Router /api/v1/public/edits [post]
func RelayEditsDoc(c *gin.Context) {}

// RelayEmbeddingsDoc godoc
// @Summary Embeddings (OpenAI compatible)
// @Tags public
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.OpenAIEmbeddingsRequest true "Embeddings request"
// @Success 200 {object} docs.OpenAIEmbeddingsResponse
// @Failure 400 {object} docs.OpenAIErrorResponse
// @Failure 401 {object} docs.OpenAIErrorResponse
// @Router /api/v1/public/embeddings [post]
func RelayEmbeddingsDoc(c *gin.Context) {}

// RelayEmbeddingsByEngineDoc godoc
// @Summary Embeddings by engine (OpenAI compatible)
// @Tags public
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param model path string true "Engine model ID"
// @Param body body docs.OpenAIEmbeddingsRequest true "Embeddings request"
// @Success 200 {object} docs.OpenAIEmbeddingsResponse
// @Failure 400 {object} docs.OpenAIErrorResponse
// @Failure 401 {object} docs.OpenAIErrorResponse
// @Router /api/v1/public/engines/{model}/embeddings [post]
func RelayEmbeddingsByEngineDoc(c *gin.Context) {}

// RelayModerationsDoc godoc
// @Summary Moderations (OpenAI compatible)
// @Tags public
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.OpenAIModerationRequest true "Moderations request"
// @Success 200 {object} docs.OpenAIModerationResponse
// @Failure 400 {object} docs.OpenAIErrorResponse
// @Failure 401 {object} docs.OpenAIErrorResponse
// @Router /api/v1/public/moderations [post]
func RelayModerationsDoc(c *gin.Context) {}

// RelayImagesGenerationsDoc godoc
// @Summary Image generations (OpenAI compatible)
// @Tags public
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body docs.OpenAIImageGenerationRequest true "Image generation request"
// @Success 200 {object} docs.OpenAIImageResponse
// @Failure 400 {object} docs.OpenAIErrorResponse
// @Failure 401 {object} docs.OpenAIErrorResponse
// @Router /api/v1/public/images/generations [post]
func RelayImagesGenerationsDoc(c *gin.Context) {}

// RelayAudioTranscriptionsDoc godoc
// @Summary Audio transcriptions (OpenAI compatible)
// @Tags public
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Audio file"
// @Param model formData string true "Model ID (e.g., whisper-1)"
// @Param language formData string false "Language (ISO-639-1)"
// @Param prompt formData string false "Prompt"
// @Param response_format formData string false "Response format (json, text, srt, vtt, verbose_json)"
// @Param temperature formData number false "Sampling temperature"
// @Success 200 {object} docs.OpenAIAudioTranscriptionResponse
// @Failure 400 {object} docs.OpenAIErrorResponse
// @Failure 401 {object} docs.OpenAIErrorResponse
// @Router /api/v1/public/audio/transcriptions [post]
func RelayAudioTranscriptionsDoc(c *gin.Context) {}

// RelayAudioTranslationsDoc godoc
// @Summary Audio translations (OpenAI compatible)
// @Tags public
// @Security BearerAuth
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Audio file"
// @Param model formData string true "Model ID (e.g., whisper-1)"
// @Param prompt formData string false "Prompt"
// @Param response_format formData string false "Response format (json, text, srt, vtt, verbose_json)"
// @Param temperature formData number false "Sampling temperature"
// @Success 200 {object} docs.OpenAIAudioTranscriptionResponse
// @Failure 400 {object} docs.OpenAIErrorResponse
// @Failure 401 {object} docs.OpenAIErrorResponse
// @Router /api/v1/public/audio/translations [post]
func RelayAudioTranslationsDoc(c *gin.Context) {}

// RelayAudioSpeechDoc godoc
// @Summary Audio speech (OpenAI compatible)
// @Tags public
// @Security BearerAuth
// @Accept json
// @Produce application/octet-stream
// @Param body body docs.OpenAITextToSpeechRequest true "Text-to-speech request"
// @Success 200 {file} file
// @Failure 400 {object} docs.OpenAIErrorResponse
// @Failure 401 {object} docs.OpenAIErrorResponse
// @Router /api/v1/public/audio/speech [post]
func RelayAudioSpeechDoc(c *gin.Context) {}
