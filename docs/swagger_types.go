package docs

// StandardResponse represents common API response.
type StandardResponse struct {
	Success   bool        `json:"success,omitempty"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Code      int         `json:"code,omitempty"`
	Timestamp int64       `json:"timestamp,omitempty"`
}

// ErrorResponse represents a common error response.
type ErrorResponse struct {
	Success   bool   `json:"success,omitempty"`
	Message   string `json:"message"`
	Code      int    `json:"code,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
}

// OpenAIErrorResponse represents OpenAI-compatible error response.
type OpenAIErrorResponse struct {
	Error OpenAIError `json:"error"`
}

// OpenAIError is the inner error object for OpenAI-compatible responses.
type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
	Code    string `json:"code"`
}

// --- Auth / User / Admin request payloads ---

type WalletChallengeRequest struct {
	Address string `json:"address" example:"0x1111111111111111111111111111111111111111"`
	ChainID string `json:"chain_id,omitempty" example:"1"`
}

type WalletLoginRequest struct {
	Address   string `json:"address" example:"0x1111111111111111111111111111111111111111"`
	Signature string `json:"signature" example:"0xabcdef..."`
	Nonce     string `json:"nonce,omitempty" example:"abc123"`
	ChainID   string `json:"chain_id,omitempty" example:"1"`
	Message   string `json:"message,omitempty" example:"Sign in to Router"`
}

type OptionUpdateRequest struct {
	Key   string `json:"key" example:"SystemName"`
	Value string `json:"value" example:"Router"`
}

type UserRegisterRequest struct {
	Username    string `json:"username" example:"alice"`
	Password    string `json:"password" example:"P@ssw0rd1"`
	Email       string `json:"email,omitempty" example:"alice@example.com"`
	DisplayName string `json:"display_name,omitempty" example:"Alice"`
	AffCode     string `json:"aff_code,omitempty" example:"ABCD"`
}

type UserSelfUpdateRequest struct {
	Username    string `json:"username,omitempty" example:"alice"`
	DisplayName string `json:"display_name,omitempty" example:"Alice"`
	Password    string `json:"password,omitempty" example:"NewPass123"`
}

type AdminUserUpdateRequest struct {
	ID            int    `json:"id" example:"123"`
	Username      string `json:"username,omitempty" example:"alice"`
	DisplayName   string `json:"display_name,omitempty" example:"Alice"`
	Role          int    `json:"role,omitempty" example:"1"`
	Status        int    `json:"status,omitempty" example:"1"`
	Quota         int64  `json:"quota,omitempty" example:"100000"`
	UsedQuota     int64  `json:"used_quota,omitempty" example:"20000"`
	Email         string `json:"email,omitempty" example:"alice@example.com"`
	Group         string `json:"group,omitempty" example:"default"`
	Password      string `json:"password,omitempty" example:"NewPass123"`
	WalletAddress string `json:"wallet_address,omitempty" example:"0x1111111111111111111111111111111111111111"`
}

type AdminCreateUserRequest struct {
	Username    string `json:"username" example:"alice"`
	Password    string `json:"password" example:"P@ssw0rd1"`
	DisplayName string `json:"display_name,omitempty" example:"Alice"`
}

type AdminManageUserRequest struct {
	Username string `json:"username" example:"alice"`
	Action   string `json:"action" example:"disable"`
}

type UserTopUpRequest struct {
	Key string `json:"key" example:"redeem-xxxx-xxxx"`
}

type TokenCreateRequest struct {
	Name           string `json:"name" example:"default"`
	ExpiredTime    int64  `json:"expired_time,omitempty" example:"-1"`
	RemainQuota    int64  `json:"remain_quota,omitempty" example:"100000"`
	UnlimitedQuota bool   `json:"unlimited_quota,omitempty" example:"false"`
	Models         string `json:"models,omitempty" example:"gpt-4o-mini,gpt-4o"`
	Subnet         string `json:"subnet,omitempty" example:"192.168.0.0/16"`
}

type TokenUpdateRequest struct {
	ID             int    `json:"id" example:"1"`
	Status         int    `json:"status,omitempty" example:"1"`
	Name           string `json:"name,omitempty" example:"default"`
	ExpiredTime    int64  `json:"expired_time,omitempty" example:"-1"`
	RemainQuota    int64  `json:"remain_quota,omitempty" example:"100000"`
	UnlimitedQuota bool   `json:"unlimited_quota,omitempty" example:"false"`
	Models         string `json:"models,omitempty" example:"gpt-4o-mini,gpt-4o"`
	Subnet         string `json:"subnet,omitempty" example:"192.168.0.0/16"`
}

type ChannelCreateRequest struct {
	Type            int    `json:"type" example:"50"`
	Key             string `json:"key" example:"sk-***"`
	Status          int    `json:"status,omitempty" example:"1"`
	Name            string `json:"name,omitempty" example:"OpenAI"`
	Weight          int    `json:"weight,omitempty" example:"0"`
	BaseURL         string `json:"base_url,omitempty" example:"https://api.openai.com"`
	Models          string `json:"models,omitempty" example:"gpt-4o-mini,gpt-4o"`
	Group           string `json:"group,omitempty" example:"default"`
	ModelMapping    string `json:"model_mapping,omitempty" example:"{}"`
	Priority        int64  `json:"priority,omitempty" example:"0"`
	Config          string `json:"config,omitempty" example:"{}"`
	SystemPrompt    string `json:"system_prompt,omitempty" example:""`
	ModelRatio      string `json:"model_ratio,omitempty" example:"{}"`
	CompletionRatio string `json:"completion_ratio,omitempty" example:"{}"`
}

type ChannelUpdateRequest struct {
	ID              int    `json:"id" example:"1"`
	Type            int    `json:"type,omitempty" example:"50"`
	Key             string `json:"key,omitempty" example:"sk-***"`
	Status          int    `json:"status,omitempty" example:"1"`
	Name            string `json:"name,omitempty" example:"OpenAI"`
	Weight          int    `json:"weight,omitempty" example:"0"`
	BaseURL         string `json:"base_url,omitempty" example:"https://api.openai.com"`
	Models          string `json:"models,omitempty" example:"gpt-4o-mini,gpt-4o"`
	Group           string `json:"group,omitempty" example:"default"`
	ModelMapping    string `json:"model_mapping,omitempty" example:"{}"`
	Priority        int64  `json:"priority,omitempty" example:"0"`
	Config          string `json:"config,omitempty" example:"{}"`
	SystemPrompt    string `json:"system_prompt,omitempty" example:""`
	ModelRatio      string `json:"model_ratio,omitempty" example:"{}"`
	CompletionRatio string `json:"completion_ratio,omitempty" example:"{}"`
}

type RedemptionCreateRequest struct {
	Name  string `json:"name" example:"InviteBonus"`
	Count int    `json:"count" example:"5"`
	Quota int64  `json:"quota" example:"100000"`
}

type RedemptionUpdateRequest struct {
	ID     int   `json:"id" example:"1"`
	Name   string `json:"name,omitempty" example:"InviteBonus"`
	Quota  int64 `json:"quota,omitempty" example:"100000"`
	Status int   `json:"status,omitempty" example:"1"`
}

// --- OpenAI-compatible models ---

type OpenAIModelPermission struct {
	ID                 string  `json:"id" example:"modelperm-abc123"`
	Object             string  `json:"object" example:"model_permission"`
	Created            int     `json:"created" example:"1626777600"`
	AllowCreateEngine  bool    `json:"allow_create_engine" example:"true"`
	AllowSampling      bool    `json:"allow_sampling" example:"true"`
	AllowLogprobs      bool    `json:"allow_logprobs" example:"true"`
	AllowSearchIndices bool    `json:"allow_search_indices" example:"false"`
	AllowView          bool    `json:"allow_view" example:"true"`
	AllowFineTuning    bool    `json:"allow_fine_tuning" example:"false"`
	Organization       string  `json:"organization" example:"*"`
	Group              *string `json:"group,omitempty" example:"default"`
	IsBlocking         bool    `json:"is_blocking" example:"false"`
}

type OpenAIModel struct {
	ID         string                 `json:"id" example:"gpt-4o-mini"`
	Object     string                 `json:"object" example:"model"`
	Created    int                    `json:"created" example:"1626777600"`
	OwnedBy    string                 `json:"owned_by" example:"openai"`
	Permission []OpenAIModelPermission `json:"permission,omitempty"`
	Root       string                 `json:"root" example:"gpt-4o-mini"`
	Parent     *string                `json:"parent,omitempty" example:""`
}

type OpenAIModelListResponse struct {
	Object string        `json:"object" example:"list"`
	Data   []OpenAIModel `json:"data"`
}

// --- OpenAI-compatible common types ---

type OpenAIUsage struct {
	PromptTokens     int                         `json:"prompt_tokens" example:"12"`
	CompletionTokens int                         `json:"completion_tokens" example:"34"`
	TotalTokens      int                         `json:"total_tokens" example:"46"`
	CompletionTokensDetails *OpenAICompletionTokensDetails `json:"completion_tokens_details,omitempty"`
}

type OpenAICompletionTokensDetails struct {
	ReasoningTokens          int `json:"reasoning_tokens,omitempty" example:"5"`
	AcceptedPredictionTokens int `json:"accepted_prediction_tokens,omitempty" example:"0"`
	RejectedPredictionTokens int `json:"rejected_prediction_tokens,omitempty" example:"0"`
}

type OpenAIImageURL struct {
	URL    string `json:"url,omitempty" example:"https://example.com/image.png"`
	Detail string `json:"detail,omitempty" example:"high"`
}

type OpenAIInputAudio struct {
	Data   string `json:"data,omitempty" example:"base64-audio"`
	Format string `json:"format,omitempty" example:"wav"`
}

type OpenAIChatContentPart struct {
	Type       string            `json:"type" example:"text"`
	Text       string            `json:"text,omitempty" example:"Hello"`
	ImageURL   *OpenAIImageURL   `json:"image_url,omitempty"`
	InputAudio *OpenAIInputAudio `json:"input_audio,omitempty"`
}

type OpenAIChatMessage struct {
	Role       string               `json:"role" example:"user"`
	Content    []OpenAIChatContentPart `json:"content"`
	Name       string               `json:"name,omitempty" example:""`
	ToolCalls  []OpenAIToolCall      `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty" example:""`
}

type OpenAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty" example:"true"`
}

type OpenAIResponseFormat struct {
	Type      string                  `json:"type" example:"json_object"`
	JSONSchema *OpenAIResponseJSONSchema `json:"json_schema,omitempty"`
}

type OpenAIResponseJSONSchema struct {
	Name        string          `json:"name,omitempty" example:"response"`
	Description string          `json:"description,omitempty" example:"Structured response"`
	Schema      OpenAIJSONSchema `json:"schema,omitempty"`
	Strict      bool            `json:"strict,omitempty" example:"true"`
}

type OpenAIJSONSchema struct {
	Type                   string                     `json:"type,omitempty" example:"object"`
	Properties             []OpenAIJSONSchemaProperty `json:"properties,omitempty"`
	Required               []string                   `json:"required,omitempty" example:"[\"answer\"]"`
	AdditionalProperties   bool                       `json:"additional_properties,omitempty" example:"false"`
}

type OpenAIJSONSchemaProperty struct {
	Name        string                    `json:"name" example:"answer"`
	Schema      OpenAIJSONSchemaPropertySchema `json:"schema"`
}

type OpenAIJSONSchemaPropertySchema struct {
	Type        string   `json:"type,omitempty" example:"string"`
	Description string   `json:"description,omitempty" example:"Answer text"`
	Enum        []string `json:"enum,omitempty" example:"[\"yes\",\"no\"]"`
}

type OpenAITool struct {
	Type     string        `json:"type" example:"function"`
	Function OpenAIFunction `json:"function"`
}

type OpenAIFunction struct {
	Name        string         `json:"name" example:"get_weather"`
	Description string         `json:"description,omitempty" example:"Get weather by city"`
	Parameters  OpenAIJSONSchema `json:"parameters,omitempty"`
}

type OpenAIToolChoice struct {
	Type     string               `json:"type,omitempty" example:"function"`
	Function OpenAIToolChoiceFunction `json:"function,omitempty"`
}

type OpenAIToolChoiceFunction struct {
	Name string `json:"name,omitempty" example:"get_weather"`
}

type OpenAIToolCall struct {
	ID       string                `json:"id,omitempty" example:"call_123"`
	Type     string                `json:"type,omitempty" example:"function"`
	Function OpenAIToolCallFunction `json:"function"`
}

type OpenAIToolCallFunction struct {
	Name      string `json:"name,omitempty" example:"get_weather"`
	Arguments string `json:"arguments,omitempty" example:"{\"city\":\"Paris\"}"`
}

// --- Chat completions ---

type OpenAIChatCompletionsRequest struct {
	Model               string                `json:"model" example:"gpt-4o-mini"`
	Messages            []OpenAIChatMessage   `json:"messages"`
	MaxTokens           int                   `json:"max_tokens,omitempty" example:"128"`
	MaxCompletionTokens int                   `json:"max_completion_tokens,omitempty" example:"128"`
	Temperature         float64               `json:"temperature,omitempty" example:"0.7"`
	TopP                float64               `json:"top_p,omitempty" example:"0.9"`
	N                   int                   `json:"n,omitempty" example:"1"`
	Stream              bool                  `json:"stream,omitempty" example:"false"`
	Stop                []string              `json:"stop,omitempty" example:"[\"\\n\\n\"]"`
	PresencePenalty     float64               `json:"presence_penalty,omitempty" example:"0"`
	FrequencyPenalty    float64               `json:"frequency_penalty,omitempty" example:"0"`
	Logprobs            bool                  `json:"logprobs,omitempty" example:"false"`
	TopLogprobs         int                   `json:"top_logprobs,omitempty" example:"0"`
	ResponseFormat      *OpenAIResponseFormat `json:"response_format,omitempty"`
	Tools               []OpenAITool          `json:"tools,omitempty"`
	ToolChoice          *OpenAIToolChoice     `json:"tool_choice,omitempty"`
	User                string                `json:"user,omitempty" example:"user-123"`
	Seed                int64                 `json:"seed,omitempty" example:"42"`
	StreamOptions       *OpenAIStreamOptions  `json:"stream_options,omitempty"`
	ParallelToolCalls   bool                  `json:"parallel_tool_calls,omitempty" example:"true"`
	ReasoningEffort     string                `json:"reasoning_effort,omitempty" example:"medium"`
	Modalities          []string              `json:"modalities,omitempty" example:"[\"text\"]"`
	Audio               *OpenAIAudioOutput    `json:"audio,omitempty"`
}

type OpenAIAudioOutput struct {
	Voice  string `json:"voice,omitempty" example:"alloy"`
	Format string `json:"format,omitempty" example:"wav"`
}

type OpenAIChatCompletionsResponse struct {
	ID      string                     `json:"id" example:"chatcmpl-123"`
	Object  string                     `json:"object" example:"chat.completion"`
	Created int64                      `json:"created" example:"1700000000"`
	Model   string                     `json:"model" example:"gpt-4o-mini"`
	Choices []OpenAIChatCompletionChoice `json:"choices"`
	Usage   OpenAIUsage                `json:"usage"`
}

type OpenAIChatCompletionChoice struct {
	Index        int                     `json:"index" example:"0"`
	Message      OpenAIChatCompletionMessage `json:"message"`
	FinishReason string                  `json:"finish_reason" example:"stop"`
}

type OpenAIChatCompletionMessage struct {
	Role      string                 `json:"role" example:"assistant"`
	Content   string                 `json:"content" example:"Hello!"`
	ToolCalls []OpenAIToolCall        `json:"tool_calls,omitempty"`
}

// --- Edits ---

type OpenAIEditRequest struct {
	Model       string  `json:"model" example:"gpt-3.5-turbo-instruct"`
	Input       string  `json:"input,omitempty" example:"What day of the wek is it?"`
	Instruction string  `json:"instruction" example:"Fix spelling"`
	N           int     `json:"n,omitempty" example:"1"`
	Temperature float64 `json:"temperature,omitempty" example:"0.7"`
	TopP        float64 `json:"top_p,omitempty" example:"0.9"`
}

type OpenAIEditResponse struct {
	Object  string              `json:"object" example:"edit"`
	Created int64               `json:"created" example:"1700000000"`
	Choices []OpenAIEditChoice  `json:"choices"`
	Usage   OpenAIUsage         `json:"usage"`
}

type OpenAIEditChoice struct {
	Index int    `json:"index" example:"0"`
	Text  string `json:"text" example:"What day of the week is it?"`
}

// --- Completions ---

type OpenAICompletionsRequest struct {
	Model            string   `json:"model" example:"gpt-3.5-turbo-instruct"`
	Prompt           []string `json:"prompt" example:"[\"Hello\"]"`
	MaxTokens        int      `json:"max_tokens,omitempty" example:"16"`
	Temperature      float64  `json:"temperature,omitempty" example:"0.7"`
	TopP             float64  `json:"top_p,omitempty" example:"0.9"`
	N                int      `json:"n,omitempty" example:"1"`
	Stream           bool     `json:"stream,omitempty" example:"false"`
	Stop             []string `json:"stop,omitempty" example:"[\"\\n\"]"`
	PresencePenalty  float64  `json:"presence_penalty,omitempty" example:"0"`
	FrequencyPenalty float64  `json:"frequency_penalty,omitempty" example:"0"`
	BestOf           int      `json:"best_of,omitempty" example:"1"`
	User             string   `json:"user,omitempty" example:"user-123"`
}

type OpenAICompletionsResponse struct {
	ID      string                     `json:"id" example:"cmpl-123"`
	Object  string                     `json:"object" example:"text_completion"`
	Created int64                      `json:"created" example:"1700000000"`
	Model   string                     `json:"model" example:"gpt-3.5-turbo-instruct"`
	Choices []OpenAICompletionsChoice  `json:"choices"`
	Usage   OpenAIUsage                `json:"usage"`
}

type OpenAICompletionsChoice struct {
	Index        int    `json:"index" example:"0"`
	Text         string `json:"text" example:"Hello world"`
	FinishReason string `json:"finish_reason" example:"stop"`
}

// --- Embeddings ---

type OpenAIEmbeddingsRequest struct {
	Model          string   `json:"model" example:"text-embedding-3-small"`
	Input          []string `json:"input" example:"[\"Hello world\"]"`
	EncodingFormat string   `json:"encoding_format,omitempty" example:"float"`
	Dimensions     int      `json:"dimensions,omitempty" example:"1536"`
	User           string   `json:"user,omitempty" example:"user-123"`
}

type OpenAIEmbeddingsResponse struct {
	Object string                 `json:"object" example:"list"`
	Data   []OpenAIEmbeddingItem  `json:"data"`
	Model  string                 `json:"model" example:"text-embedding-3-small"`
	Usage  OpenAIUsage            `json:"usage"`
}

type OpenAIEmbeddingItem struct {
	Object    string    `json:"object" example:"embedding"`
	Index     int       `json:"index" example:"0"`
	Embedding []float64 `json:"embedding"`
}

// --- Moderations ---

type OpenAIModerationRequest struct {
	Model string   `json:"model,omitempty" example:"omni-moderation-latest"`
	Input []string `json:"input" example:"[\"I want to hurt someone\"]"`
}

type OpenAIModerationResponse struct {
	ID      string                   `json:"id" example:"modr-123"`
	Model   string                   `json:"model" example:"omni-moderation-latest"`
	Results []OpenAIModerationResult `json:"results"`
}

type OpenAIModerationResult struct {
	Flagged        bool                           `json:"flagged" example:"false"`
	Categories     OpenAIModerationCategories     `json:"categories"`
	CategoryScores OpenAIModerationCategoryScores `json:"category_scores"`
}

type OpenAIModerationCategories struct {
	Hate                   bool `json:"hate" example:"false"`
	HateThreatening        bool `json:"hate_threatening" example:"false"`
	Harassment             bool `json:"harassment" example:"false"`
	HarassmentThreatening  bool `json:"harassment_threatening" example:"false"`
	SelfHarm               bool `json:"self_harm" example:"false"`
	SelfHarmIntent         bool `json:"self_harm_intent" example:"false"`
	SelfHarmInstructions   bool `json:"self_harm_instructions" example:"false"`
	Sexual                 bool `json:"sexual" example:"false"`
	SexualMinors           bool `json:"sexual_minors" example:"false"`
	Violence               bool `json:"violence" example:"false"`
	ViolenceGraphic        bool `json:"violence_graphic" example:"false"`
	Illicit                bool `json:"illicit" example:"false"`
	IllicitViolent         bool `json:"illicit_violent" example:"false"`
}

type OpenAIModerationCategoryScores struct {
	Hate                   float64 `json:"hate" example:"0"`
	HateThreatening        float64 `json:"hate_threatening" example:"0"`
	Harassment             float64 `json:"harassment" example:"0"`
	HarassmentThreatening  float64 `json:"harassment_threatening" example:"0"`
	SelfHarm               float64 `json:"self_harm" example:"0"`
	SelfHarmIntent         float64 `json:"self_harm_intent" example:"0"`
	SelfHarmInstructions   float64 `json:"self_harm_instructions" example:"0"`
	Sexual                 float64 `json:"sexual" example:"0"`
	SexualMinors           float64 `json:"sexual_minors" example:"0"`
	Violence               float64 `json:"violence" example:"0"`
	ViolenceGraphic        float64 `json:"violence_graphic" example:"0"`
	Illicit                float64 `json:"illicit" example:"0"`
	IllicitViolent         float64 `json:"illicit_violent" example:"0"`
}

// --- Images ---

type OpenAIImageGenerationRequest struct {
	Model          string `json:"model,omitempty" example:"gpt-image-1"`
	Prompt         string `json:"prompt" example:"A cute cat"`
	N              int    `json:"n,omitempty" example:"1"`
	Size           string `json:"size,omitempty" example:"1024x1024"`
	Quality        string `json:"quality,omitempty" example:"standard"`
	ResponseFormat string `json:"response_format,omitempty" example:"url"`
	Style          string `json:"style,omitempty" example:"vivid"`
	User           string `json:"user,omitempty" example:"user-123"`
}

type OpenAIImageResponse struct {
	Created int64          `json:"created" example:"1700000000"`
	Data    []OpenAIImageData `json:"data"`
}

type OpenAIImageData struct {
	URL           string `json:"url,omitempty" example:"https://example.com/image.png"`
	B64JSON       string `json:"b64_json,omitempty" example:"base64-image"`
	RevisedPrompt string `json:"revised_prompt,omitempty" example:"A cute cat"`
}

// --- Audio ---

type OpenAITextToSpeechRequest struct {
	Model          string  `json:"model" example:"gpt-4o-mini-tts"`
	Input          string  `json:"input" example:"Hello world"`
	Voice          string  `json:"voice" example:"alloy"`
	Speed          float64 `json:"speed,omitempty" example:"1"`
	ResponseFormat string  `json:"response_format,omitempty" example:"mp3"`
}

type OpenAIAudioTranscriptionResponse struct {
	Text     string               `json:"text" example:"Hello world"`
	Task     string               `json:"task,omitempty" example:"transcribe"`
	Language string               `json:"language,omitempty" example:"en"`
	Duration float64              `json:"duration,omitempty" example:"12.34"`
	Segments []OpenAIWhisperSegment `json:"segments,omitempty"`
}

type OpenAIWhisperSegment struct {
	ID               int     `json:"id" example:"0"`
	Seek             int     `json:"seek" example:"0"`
	Start            float64 `json:"start" example:"0.0"`
	End              float64 `json:"end" example:"1.2"`
	Text             string  `json:"text" example:"Hello"`
	Tokens           []int   `json:"tokens,omitempty"`
	Temperature      float64 `json:"temperature,omitempty" example:"0.0"`
	AvgLogprob       float64 `json:"avg_logprob,omitempty" example:"-0.1"`
	CompressionRatio float64 `json:"compression_ratio,omitempty" example:"1.0"`
	NoSpeechProb     float64 `json:"no_speech_prob,omitempty" example:"0.0"`
}
