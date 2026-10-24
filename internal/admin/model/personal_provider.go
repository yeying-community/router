package model

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/yeying-community/router/common/client"
	"github.com/yeying-community/router/common/config"
	"github.com/yeying-community/router/common/helper"
	"github.com/yeying-community/router/common/random"
	relaychannel "github.com/yeying-community/router/internal/relay/channel"
	"gorm.io/gorm"
)

const (
	PersonalProviderConnectionsTableName = "personal_provider_connections"
	PersonalModelRoutesTableName         = "personal_model_routes"

	PersonalProviderStatusEnabled  = 1
	PersonalProviderStatusDisabled = 2

	PersonalRoutePolicyPersonalFirst     = "personal_first"
	PersonalRoutePolicyPersonalOnly      = "personal_only"
	PersonalRoutePolicyCommunityFallback = "community_fallback"
	PersonalRoutePolicyCommunityOnly     = "community_only"
	PersonalRoutePolicyCommunityFirst    = "community_first"

	PersonalProviderChannelPrefix = "personal:"
	PersonalProviderSourceType    = "personal_provider"
)

// PersonalProviderConnection belongs to exactly one Router user. Credentials
// are encrypted at rest and deliberately never carry a JSON field.
type PersonalProviderConnection struct {
	Id                   string   `json:"id" gorm:"type:char(36);primaryKey"`
	UserId               string   `json:"user_id" gorm:"type:char(36);not null;index:idx_personal_provider_user_name,priority:1;index"`
	Name                 string   `json:"name" gorm:"type:varchar(96);not null;index:idx_personal_provider_user_name,priority:2"`
	Protocol             string   `json:"protocol" gorm:"type:varchar(64);not null;default:'openai'"`
	BaseURL              string   `json:"base_url" gorm:"type:text;not null;default:''"`
	CredentialEncrypted  string   `json:"-" gorm:"type:text;not null"`
	ModelsJSON           string   `json:"-" gorm:"type:text;not null;default:'[]'"`
	Models               []string `json:"models" gorm:"-"`
	Priority             int64    `json:"priority" gorm:"type:bigint;not null;default:0;index"`
	Status               int      `json:"status" gorm:"not null;default:1;index"`
	CreatedAt            int64    `json:"created_at" gorm:"bigint;not null;index"`
	UpdatedAt            int64    `json:"updated_at" gorm:"bigint;not null;index"`
	CredentialConfigured bool     `json:"credential_configured" gorm:"-"`
	LastCheckedAt        int64    `json:"last_checked_at" gorm:"bigint;not null;default:0"`
	LastCheckOK          bool     `json:"last_check_ok" gorm:"not null;default:false"`
	LastCheckError       string   `json:"last_check_error" gorm:"type:text;not null;default:''"`
}

func (PersonalProviderConnection) TableName() string { return PersonalProviderConnectionsTableName }

type PersonalModelRoute struct {
	UserId      string `json:"user_id" gorm:"type:char(36);primaryKey"`
	Model       string `json:"model" gorm:"type:varchar(255);primaryKey"`
	RoutePolicy string `json:"route_policy" gorm:"type:varchar(32);not null"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint;not null"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint;not null"`
}

func (PersonalModelRoute) TableName() string { return PersonalModelRoutesTableName }

func NormalizePersonalRoutePolicy(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case PersonalRoutePolicyPersonalOnly, PersonalRoutePolicyCommunityOnly, PersonalRoutePolicyCommunityFirst:
		return strings.TrimSpace(strings.ToLower(value))
	case PersonalRoutePolicyCommunityFallback, PersonalRoutePolicyPersonalFirst:
		return PersonalRoutePolicyPersonalFirst
	default:
		return PersonalRoutePolicyPersonalFirst
	}
}

func IsPersonalRoutePolicy(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case PersonalRoutePolicyPersonalFirst, PersonalRoutePolicyPersonalOnly, PersonalRoutePolicyCommunityOnly, PersonalRoutePolicyCommunityFirst:
		return true
	default:
		return false
	}
}

// IsPersonalProviderProtocol is intentionally narrower than Router's
// administrator-owned channel protocol set. Personal connections support the
// text protocols exposed by the user workspace, not every internal adapter.
func IsPersonalProviderProtocol(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "openai", "anthropic", "gemini", "ali", "deepseek":
		return true
	default:
		return false
	}
}

func IsPersonalProviderChannelID(channelID string) bool {
	return strings.HasPrefix(strings.TrimSpace(channelID), PersonalProviderChannelPrefix)
}

func PersonalProviderIDFromChannelID(channelID string) string {
	return strings.TrimPrefix(strings.TrimSpace(channelID), PersonalProviderChannelPrefix)
}

func normalizePersonalProviderModels(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	result := make([]string, 0, len(models))
	for _, modelName := range models {
		value := strings.TrimSpace(modelName)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (connection *PersonalProviderConnection) normalize() error {
	if connection == nil {
		return errors.New("个人渠道不能为空")
	}
	connection.Id = strings.TrimSpace(connection.Id)
	connection.UserId = strings.TrimSpace(connection.UserId)
	connection.Name = strings.TrimSpace(connection.Name)
	connection.Protocol = relaychannel.NormalizeProtocolName(connection.Protocol)
	if connection.Protocol == "" {
		connection.Protocol = "openai"
	}
	if !IsPersonalProviderProtocol(connection.Protocol) {
		return errors.New("个人渠道协议不受支持")
	}
	connection.BaseURL = strings.TrimRight(strings.TrimSpace(connection.BaseURL), "/")
	if err := client.ValidatePersonalProviderBaseURL(connection.BaseURL); err != nil {
		return err
	}
	connection.Models = normalizePersonalProviderModels(connection.Models)
	if connection.UserId == "" {
		return errors.New("用户 ID 不能为空")
	}
	if connection.Name == "" {
		return errors.New("连接名称不能为空")
	}
	if len(connection.Name) > 96 {
		return errors.New("连接名称不能超过 96 个字符")
	}
	if len(connection.Models) == 0 {
		return errors.New("至少选择一个模型")
	}
	if connection.Status == 0 {
		connection.Status = PersonalProviderStatusEnabled
	}
	if connection.Status != PersonalProviderStatusEnabled && connection.Status != PersonalProviderStatusDisabled {
		return errors.New("个人渠道状态无效")
	}
	return nil
}

func (connection *PersonalProviderConnection) hydrate() {
	var models []string
	_ = json.Unmarshal([]byte(connection.ModelsJSON), &models)
	connection.Models = normalizePersonalProviderModels(models)
	connection.CredentialConfigured = strings.TrimSpace(connection.CredentialEncrypted) != ""
}

func (connection *PersonalProviderConnection) persistModels() error {
	encoded, err := json.Marshal(normalizePersonalProviderModels(connection.Models))
	if err != nil {
		return err
	}
	connection.ModelsJSON = string(encoded)
	return nil
}

func personalCredentialKey(secret string) []byte {
	sum := sha256.Sum256([]byte("router.personal-provider-credentials.v1:" + secret))
	return sum[:]
}

func encryptPersonalProviderCredential(value string) (string, error) {
	secret := strings.TrimSpace(config.JWTSecret)
	if secret == "" {
		return "", errors.New("auth.jwt_secret 未配置，无法保存个人渠道凭据")
	}
	block, err := aes.NewCipher(personalCredentialKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(strings.TrimSpace(value)), nil)
	return "v1:" + base64.RawURLEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func decryptPersonalProviderCredential(value string) (string, error) {
	raw := strings.TrimPrefix(strings.TrimSpace(value), "v1:")
	bytes, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", errors.New("个人渠道凭据格式无效")
	}
	secrets := append([]string{strings.TrimSpace(config.JWTSecret)}, config.JWTFallbackSecrets...)
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		block, blockErr := aes.NewCipher(personalCredentialKey(secret))
		if blockErr != nil {
			continue
		}
		gcm, gcmErr := cipher.NewGCM(block)
		if gcmErr != nil || len(bytes) < gcm.NonceSize() {
			continue
		}
		plaintext, openErr := gcm.Open(nil, bytes[:gcm.NonceSize()], bytes[gcm.NonceSize():], nil)
		if openErr == nil {
			return string(plaintext), nil
		}
	}
	return "", errors.New("无法解密个人渠道凭据，请轮换凭据")
}

func ListPersonalProviderConnections(userID string) ([]PersonalProviderConnection, error) {
	rows := make([]PersonalProviderConnection, 0)
	if err := DB.Where("user_id = ?", strings.TrimSpace(userID)).Order("priority desc, updated_at desc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].hydrate()
	}
	return rows, nil
}

func ListPersonalProviderConnectionsForAdmin() ([]PersonalProviderConnection, error) {
	if DB == nil || !DB.Migrator().HasTable(&PersonalProviderConnection{}) {
		return []PersonalProviderConnection{}, nil
	}
	rows := make([]PersonalProviderConnection, 0)
	if err := DB.Order("updated_at desc, id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].hydrate()
		rows[i].CredentialConfigured = strings.TrimSpace(rows[i].CredentialEncrypted) != ""
	}
	return rows, nil
}

// ListPersonalProviderModels reads only public connection metadata. It is used
// when validating an API Token so a user can authorize a model supplied by a
// private connection even before buying a community package for that model.
func ListPersonalProviderModels(userID string) ([]string, error) {
	if DB == nil || !DB.Migrator().HasTable(&PersonalProviderConnection{}) {
		return []string{}, nil
	}
	rows := make([]PersonalProviderConnection, 0)
	if err := DB.Select("models_json").
		Where("user_id = ? AND status = ?", strings.TrimSpace(userID), PersonalProviderStatusEnabled).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	models := make([]string, 0)
	for i := range rows {
		rows[i].hydrate()
		models = append(models, rows[i].Models...)
	}
	return normalizePersonalProviderModels(models), nil
}

// MergePersonalProviderModelsIntoEntitlements adds a user's enabled personal
// connection models to the same catalog returned to Token management. It keeps
// these sources separate from billable community entitlements: no group is
// assigned and a model that is already available through a package retains its
// package provider metadata.
func MergePersonalProviderModelsIntoEntitlements(userID string, payload UserEntitlementModelsPayload) (UserEntitlementModelsPayload, error) {
	connections, err := ListPersonalProviderConnections(userID)
	if err != nil {
		return payload, err
	}
	itemByModel := make(map[string]int, len(payload.Items))
	modelSet := make(map[string]struct{}, len(payload.Models))
	for _, modelName := range payload.Models {
		normalized := strings.TrimSpace(modelName)
		if normalized != "" {
			modelSet[normalized] = struct{}{}
		}
	}
	for index, item := range payload.Items {
		if normalized := strings.TrimSpace(item.Model); normalized != "" {
			itemByModel[normalized] = index
			modelSet[normalized] = struct{}{}
		}
	}
	for _, connection := range connections {
		if connection.Status != PersonalProviderStatusEnabled {
			continue
		}
		for _, modelName := range connection.Models {
			normalizedModel := strings.TrimSpace(modelName)
			if normalizedModel == "" {
				continue
			}
			source := UserEntitlementModelSource{
				SourceType: PersonalProviderSourceType,
				SourceID:   connection.Id,
				SourceName: connection.Name,
				Priority:   int(connection.Priority),
			}
			if itemIndex, exists := itemByModel[normalizedModel]; exists {
				payload.Items[itemIndex].Sources = append(payload.Items[itemIndex].Sources, source)
				continue
			}
			payload.Items = append(payload.Items, UserAvailableModel{
				Model:         normalizedModel,
				Provider:      PersonalProviderSourceType,
				ProviderLabel: "个人渠道",
				Sources:       []UserEntitlementModelSource{source},
			})
			itemByModel[normalizedModel] = len(payload.Items) - 1
			modelSet[normalizedModel] = struct{}{}
		}
	}
	payload.Models = payload.Models[:0]
	for modelName := range modelSet {
		payload.Models = append(payload.Models, modelName)
	}
	sort.Strings(payload.Models)
	sort.SliceStable(payload.Items, func(left, right int) bool {
		return payload.Items[left].Model < payload.Items[right].Model
	})
	return payload, nil
}

func GetPersonalProviderConnection(userID string, id string) (*PersonalProviderConnection, error) {
	row := &PersonalProviderConnection{}
	if err := DB.Where("id = ? AND user_id = ?", strings.TrimSpace(id), strings.TrimSpace(userID)).First(row).Error; err != nil {
		return nil, err
	}
	row.hydrate()
	return row, nil
}

func CreatePersonalProviderConnection(connection *PersonalProviderConnection, credential string) error {
	if err := connection.normalize(); err != nil {
		return err
	}
	if strings.TrimSpace(credential) == "" {
		return errors.New("API Key 不能为空")
	}
	if err := connection.persistModels(); err != nil {
		return err
	}
	encrypted, err := encryptPersonalProviderCredential(credential)
	if err != nil {
		return err
	}
	now := helper.GetTimestamp()
	connection.Id = random.GetUUID()
	connection.CredentialEncrypted = encrypted
	connection.CreatedAt = now
	connection.UpdatedAt = now
	if err := DB.Create(connection).Error; err != nil {
		return err
	}
	connection.hydrate()
	return nil
}

func UpdatePersonalProviderConnection(connection *PersonalProviderConnection, credential *string) error {
	if err := connection.normalize(); err != nil {
		return err
	}
	if err := connection.persistModels(); err != nil {
		return err
	}
	existing := &PersonalProviderConnection{}
	if err := DB.Where("id = ? AND user_id = ?", connection.Id, connection.UserId).First(existing).Error; err != nil {
		return err
	}
	endpointChanged := existing.Protocol != connection.Protocol || existing.BaseURL != connection.BaseURL || credential != nil
	updates := map[string]any{"name": connection.Name, "protocol": connection.Protocol, "base_url": connection.BaseURL, "models_json": connection.ModelsJSON, "priority": connection.Priority, "status": connection.Status, "updated_at": helper.GetTimestamp()}
	if credential != nil {
		if strings.TrimSpace(*credential) == "" {
			return errors.New("API Key 不能为空")
		}
		encrypted, err := encryptPersonalProviderCredential(*credential)
		if err != nil {
			return err
		}
		updates["credential_encrypted"] = encrypted
	}
	if endpointChanged {
		// A test result belongs to the exact endpoint and credential that was
		// probed. Renaming, reprioritising, or enabling a connection does not
		// invalidate it; changing the endpoint, protocol, or key does.
		updates["last_checked_at"] = int64(0)
		updates["last_check_ok"] = false
		updates["last_check_error"] = ""
	}
	result := DB.Model(&PersonalProviderConnection{}).Where("id = ? AND user_id = ?", connection.Id, connection.UserId).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func DeletePersonalProviderConnection(userID string, id string) error {
	result := DB.Where("id = ? AND user_id = ?", strings.TrimSpace(id), strings.TrimSpace(userID)).Delete(&PersonalProviderConnection{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func ListPersonalModelRoutes(userID string) ([]PersonalModelRoute, error) {
	rows := make([]PersonalModelRoute, 0)
	if err := DB.Where("user_id = ?", strings.TrimSpace(userID)).Order("model asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].RoutePolicy = NormalizePersonalRoutePolicy(rows[i].RoutePolicy)
	}
	return rows, nil
}

func UpsertPersonalModelRoute(userID string, modelName string, policy string) error {
	userID, modelName = strings.TrimSpace(userID), strings.TrimSpace(modelName)
	if userID == "" || modelName == "" {
		return errors.New("用户和模型不能为空")
	}
	if !IsPersonalRoutePolicy(policy) {
		return errors.New("路由策略无效")
	}
	now := helper.GetTimestamp()
	row := PersonalModelRoute{UserId: userID, Model: modelName, RoutePolicy: NormalizePersonalRoutePolicy(policy), CreatedAt: now, UpdatedAt: now}
	return DB.Where(PersonalModelRoute{UserId: userID, Model: modelName}).Assign(map[string]any{"route_policy": row.RoutePolicy, "updated_at": now}).FirstOrCreate(&row).Error
}

func DeletePersonalModelRoute(userID string, modelName string) error {
	result := DB.Where("user_id = ? AND model = ?", strings.TrimSpace(userID), strings.TrimSpace(modelName)).Delete(&PersonalModelRoute{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func ResolvePersonalRoutePolicy(userID string, token *Token, modelName string) string {
	policy := PersonalRoutePolicyPersonalFirst
	if token != nil {
		policy = NormalizePersonalRoutePolicy(token.RoutePolicy)
	}
	row := PersonalModelRoute{}
	if err := DB.Select("route_policy").Where("user_id = ? AND model = ?", strings.TrimSpace(userID), strings.TrimSpace(modelName)).First(&row).Error; err == nil {
		modelPolicy := NormalizePersonalRoutePolicy(row.RoutePolicy)
		// A Token can carry a hard source boundary. A user-level model rule is
		// allowed to narrow a flexible Token policy, but never to widen a Token
		// that was deliberately issued as personal_only or community_only.
		if policy != PersonalRoutePolicyPersonalOnly && policy != PersonalRoutePolicyCommunityOnly {
			policy = modelPolicy
		}
	}
	return policy
}

const personalProviderVerificationTimeout = 12 * time.Second

var doPersonalProviderVerificationRequest = client.DoPersonalProviderRequest

// VerifyPersonalProviderConnection probes the protocol's model catalogue. It
// validates the stored endpoint and credential without sending an inference
// request, so a verification never consumes model tokens by design.
func VerifyPersonalProviderConnection(userID string, id string) (*PersonalProviderConnection, error) {
	connection, err := GetPersonalProviderConnection(userID, id)
	if err != nil {
		return nil, err
	}
	credential, err := decryptPersonalProviderCredential(connection.CredentialEncrypted)
	if err == nil {
		err = verifyPersonalProviderCredential(connection, credential)
	}
	now := helper.GetTimestamp()
	updates := map[string]any{"last_checked_at": now, "last_check_ok": err == nil, "last_check_error": ""}
	if err != nil {
		updates["last_check_error"] = truncatePersonalProviderCheckError(err.Error())
	}
	if updateErr := DB.Model(&PersonalProviderConnection{}).
		Where("id = ? AND user_id = ?", connection.Id, connection.UserId).
		Updates(updates).Error; updateErr != nil {
		return nil, updateErr
	}
	connection.LastCheckedAt = now
	connection.LastCheckOK = err == nil
	connection.LastCheckError = fmt.Sprint(updates["last_check_error"])
	if err != nil {
		return connection, err
	}
	return connection, nil
}

func verifyPersonalProviderCredential(connection *PersonalProviderConnection, credential string) error {
	endpoint, err := personalProviderVerificationEndpoint(connection)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), personalProviderVerificationTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("无法创建个人渠道验证请求: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	switch connection.Protocol {
	case "anthropic":
		req.Header.Set("x-api-key", credential)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "gemini":
		req.Header.Set("x-goog-api-key", credential)
	default:
		req.Header.Set("Authorization", "Bearer "+credential)
	}
	resp, err := doPersonalProviderVerificationRequest(req)
	if err != nil {
		return fmt.Errorf("无法连接个人渠道: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return errors.New("个人渠道拒绝了 API Key，请检查凭据权限或轮换 API Key")
		}
		return fmt.Errorf("个人渠道验证失败：上游返回 HTTP %d", resp.StatusCode)
	}
	return nil
}

func personalProviderVerificationEndpoint(connection *PersonalProviderConnection) (string, error) {
	if connection == nil {
		return "", errors.New("个人渠道不能为空")
	}
	baseURL := strings.TrimSpace(connection.BaseURL)
	if baseURL == "" {
		switch connection.Protocol {
		case "openai":
			baseURL = "https://api.openai.com/v1"
		case "anthropic":
			baseURL = "https://api.anthropic.com/v1"
		case "gemini":
			baseURL = "https://generativelanguage.googleapis.com/v1beta"
		case "ali":
			baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		case "deepseek":
			baseURL = "https://api.deepseek.com"
		default:
			return "", errors.New("个人渠道协议不受支持")
		}
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("个人渠道 Base URL 无效: %w", err)
	}
	path := strings.TrimRight(parsed.Path, "/")
	switch connection.Protocol {
	case "openai", "anthropic":
		if !strings.HasSuffix(path, "/v1") {
			path += "/v1"
		}
	case "gemini":
		if !strings.Contains(path, "/v1") {
			path += "/v1beta"
		}
	case "ali":
		if !strings.Contains(path, "/compatible-mode/v1") {
			path += "/compatible-mode/v1"
		}
	case "deepseek":
		// DeepSeek exposes its catalogue at the service root /models endpoint.
	default:
		return "", errors.New("个人渠道协议不受支持")
	}
	parsed.Path = strings.TrimRight(path, "/") + "/models"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func truncatePersonalProviderCheckError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 512 {
		return value
	}
	return value[:512]
}

func ListPersonalProviderChannelsForModel(userID string, modelName string) ([]*Channel, error) {
	connections, err := ListPersonalProviderConnections(userID)
	if err != nil {
		return nil, err
	}
	channels := make([]*Channel, 0)
	for _, connection := range connections {
		if connection.Status != PersonalProviderStatusEnabled {
			continue
		}
		matched := false
		for _, candidate := range connection.Models {
			if candidate == strings.TrimSpace(modelName) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		credential, decryptErr := decryptPersonalProviderCredential(connection.CredentialEncrypted)
		if decryptErr != nil {
			return nil, fmt.Errorf("个人渠道 %s 的凭据不可用: %w", connection.Name, decryptErr)
		}
		baseURL := connection.BaseURL
		channel := &Channel{Id: PersonalProviderChannelPrefix + connection.Id, Name: "personal-" + connection.Id, Protocol: connection.Protocol, Key: credential, BaseURL: &baseURL, Status: ChannelStatusEnabled, Priority: &connection.Priority, PersonalProviderName: connection.Name}
		rows := BuildDefaultChannelModelsWithProtocol(connection.Models, channel.GetChannelProtocol())
		for i := range rows {
			rows[i].ChannelId = channel.Id
			rows[i].Type = "text"
		}
		channel.ChannelModels = rows
		channels = append(channels, channel)
	}
	return channels, nil
}

func ResolvePersonalProviderChannel(userID string, channelID string) (*Channel, error) {
	if !IsPersonalProviderChannelID(channelID) {
		return nil, gorm.ErrRecordNotFound
	}
	connection, err := GetPersonalProviderConnection(userID, PersonalProviderIDFromChannelID(channelID))
	if err != nil {
		return nil, err
	}
	if connection.Status != PersonalProviderStatusEnabled {
		return nil, errors.New("个人渠道已禁用")
	}
	credential, err := decryptPersonalProviderCredential(connection.CredentialEncrypted)
	if err != nil {
		return nil, err
	}
	baseURL := connection.BaseURL
	channel := &Channel{Id: PersonalProviderChannelPrefix + connection.Id, Name: "personal-" + connection.Id, Protocol: connection.Protocol, Key: credential, BaseURL: &baseURL, Status: ChannelStatusEnabled, Priority: &connection.Priority, PersonalProviderName: connection.Name}
	rows := BuildDefaultChannelModelsWithProtocol(connection.Models, channel.GetChannelProtocol())
	for i := range rows {
		rows[i].ChannelId = channel.Id
		rows[i].Type = "text"
	}
	channel.ChannelModels = rows
	return channel, nil
}
