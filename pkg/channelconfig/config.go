/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

// Package channelconfig parses and validates the declarative channel
// bootstrap file consumed by `cuberouter channels import`.
//
// One file describes channels and how their models are discovered. Optional
// price rules are parsed here as well. The file is data only: parsing,
// validation and price conversion live here so they can be tested without a
// database, while applying the plan to the database lives in the controller
// layer.
package channelconfig

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/constant"

	"gopkg.in/yaml.v3"
)

// CurrentVersion is the only config schema version this build understands.
const CurrentVersion = 1

// Config is the root document of a channel bootstrap file.
type Config struct {
	Version  int       `yaml:"version" json:"version"`
	Channels []Channel `yaml:"channels" json:"channels"`
	Pricing  *Pricing  `yaml:"pricing" json:"pricing"`
}

// Channel describes one channel to create or update. Channels are matched by
// name: an existing channel with the same name is updated, otherwise a new one
// is created.
type Channel struct {
	Name string `yaml:"name" json:"name"`
	// Type accepts a channel type ID (64) or a name ("cubestack", "openai").
	Type any `yaml:"type" json:"type"`

	// BaseURL is the upstream API address. When empty, the provider default
	// from constant.ChannelBaseURLs is used; providers without a default
	// (for example cubestack) require it.
	BaseURL string `yaml:"base_url" json:"base_url"`

	// Key carries exactly one upstream credential. Use Keys for several
	// credentials or KeyEnv to read one from the environment.
	Key    string   `yaml:"key" json:"key"`
	KeyEnv string   `yaml:"key_env" json:"key_env"`
	Keys   []string `yaml:"keys" json:"keys"`

	Group        string  `yaml:"group" json:"group"`
	Priority     *int64  `yaml:"priority" json:"priority"`
	Weight       *uint   `yaml:"weight" json:"weight"`
	Status       *int    `yaml:"status" json:"status"`
	Tag          *string `yaml:"tag" json:"tag"`
	Remark       *string `yaml:"remark" json:"remark"`
	TestModel    *string `yaml:"test_model" json:"test_model"`
	AutoBan      *int    `yaml:"auto_ban" json:"auto_ban"`
	MultiKeyMode string  `yaml:"multi_key_mode" json:"multi_key_mode"`
	Other        string  `yaml:"other" json:"other"`

	Models *Models `yaml:"models" json:"models"`

	ModelMapping   map[string]string `yaml:"model_mapping" json:"model_mapping"`
	Overrides      map[string]any    `yaml:"overrides" json:"overrides"`
	HeaderOverride map[string]any    `yaml:"header_override" json:"header_override"`
	Proxy          string            `yaml:"proxy" json:"proxy"`
	Settings       map[string]any    `yaml:"settings" json:"settings"`
	OtherSettings  map[string]any    `yaml:"other_settings" json:"other_settings"`

	Pricing *Pricing `yaml:"pricing" json:"pricing"`
}

// Models controls how the channel's model list is built.
type Models struct {
	// Fetch pulls the model list from the upstream /v1/models endpoint.
	// Defaults to true.
	Fetch *bool `yaml:"fetch" json:"fetch"`
	// List is always added to the channel and is not filtered.
	List []string `yaml:"list" json:"list"`
	// Include/exclude are glob patterns ("qwen-*", "*-embedding") applied to
	// the fetched models only.
	Include []string `yaml:"include" json:"include"`
	Exclude []string `yaml:"exclude" json:"exclude"`
}

// Pricing is a set of model prices in USD.
type Pricing struct {
	// Defaults applies to every model of the channel (or of every channel
	// when used at the top level).
	Defaults *Price `yaml:"defaults" json:"defaults"`
	// Models overrides individual model names and has priority over Defaults.
	Models map[string]Price `yaml:"models" json:"models"`
}

// Price is one model's price. Token prices are USD per 1M tokens; PerRequest
// is USD per request and cannot be combined with token prices.
type Price struct {
	Input      *float64 `yaml:"input" json:"input"`
	Output     *float64 `yaml:"output" json:"output"`
	CacheRead  *float64 `yaml:"cache_read" json:"cache_read"`
	CacheWrite *float64 `yaml:"cache_write" json:"cache_write"`
	PerRequest *float64 `yaml:"per_request" json:"per_request"`
}

// ModelPrice is a Price converted to the units cuberouter stores.
type ModelPrice struct {
	// ModelRatio is USD/1M input price divided by 2 (1 ratio == $2/1M).
	ModelRatio float64
	// CompletionRatio is output price divided by input price.
	CompletionRatio float64
	// CacheRatio and CreateCacheRatio are cache read/write prices divided by
	// the input price; nil means the global default applies.
	CacheRatio       *float64
	CreateCacheRatio *float64
	// PerRequest is USD per request; when set the model is billed per call
	// and the ratio fields are unused.
	PerRequest *float64
}

// Equal reports whether two converted prices describe the same billing.
func (p ModelPrice) Equal(other ModelPrice) bool {
	if p.ModelRatio != other.ModelRatio || p.CompletionRatio != other.CompletionRatio {
		return false
	}
	return equalFloatPointer(p.CacheRatio, other.CacheRatio) &&
		equalFloatPointer(p.CreateCacheRatio, other.CreateCacheRatio) &&
		equalFloatPointer(p.PerRequest, other.PerRequest)
}

// PriceLayer is one step of the price precedence chain. Defaults applies to
// every model in scope; Models overrides named models.
type PriceLayer struct {
	Defaults *Price
	Models   map[string]Price
}

// Parse decodes, normalizes and validates a channel bootstrap document.
// Unknown fields are rejected so typos fail loudly instead of being ignored.
func Parse(data []byte) (*Config, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	config := &Config{}
	if err := decoder.Decode(config); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("channel config file is empty")
		}
		return nil, fmt.Errorf("parse channel config: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("channel config must contain a single YAML document")
		}
		return nil, fmt.Errorf("parse channel config: %w", err)
	}

	config.Normalize()
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return config, nil
}

// Normalize fills in defaults. It never fails; Validate reports bad input.
func (c *Config) Normalize() {
	if c.Version == 0 {
		c.Version = CurrentVersion
	}
	for i := range c.Channels {
		c.Channels[i].normalize()
	}
}

// Validate checks the whole document and returns the first problem found.
func (c *Config) Validate() error {
	if c.Version != CurrentVersion {
		return fmt.Errorf("unsupported config version %d (this build supports version %d)", c.Version, CurrentVersion)
	}
	if len(c.Channels) == 0 {
		return errors.New("config defines no channels")
	}
	sawChannel := make(map[string]int, len(c.Channels))
	for i := range c.Channels {
		channel := &c.Channels[i]
		if err := channel.validate(); err != nil {
			return fmt.Errorf("channels[%d] (%s): %w", i, channel.Name, err)
		}
		if first, exists := sawChannel[channel.Name]; exists {
			return fmt.Errorf("duplicate channel name %q at channels[%d] and channels[%d]", channel.Name, first, i)
		}
		sawChannel[channel.Name] = i
	}
	if c.Pricing != nil {
		if err := c.Pricing.validate(); err != nil {
			return fmt.Errorf("pricing: %w", err)
		}
	}
	return nil
}

func (c *Channel) normalize() {
	c.Name = strings.TrimSpace(c.Name)
	if c.Group == "" {
		c.Group = "default"
	}
	if c.Models == nil {
		c.Models = &Models{}
	}
	if c.Models.Fetch == nil {
		fetch := true
		c.Models.Fetch = &fetch
	}
	if c.MultiKeyMode == "" {
		c.MultiKeyMode = string(constant.MultiKeyModeRandom)
	}
}

// ChannelKeys returns the credentials configured for this channel. KeyEnv is
// read at call time so the environment can be populated after parsing.
func (c *Channel) ChannelKeys() []string {
	if c.KeyEnv != "" && c.Key == "" && len(c.Keys) == 0 {
		if value := strings.TrimSpace(os.Getenv(c.KeyEnv)); value != "" {
			return []string{value}
		}
	}
	keys := make([]string, 0, len(c.Keys)+1)
	if key := strings.TrimSpace(c.Key); key != "" {
		keys = append(keys, key)
	}
	for _, key := range c.Keys {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	return keys
}

func (c *Channel) validate() error {
	if c.Name == "" {
		return errors.New("name is required")
	}
	channelType, err := ResolveChannelType(c.Type)
	if err != nil {
		return err
	}
	keySources := 0
	if strings.TrimSpace(c.Key) != "" {
		keySources++
	}
	if len(c.Keys) > 0 {
		keySources++
	}
	if c.KeyEnv != "" {
		keySources++
	}
	if keySources > 1 {
		return errors.New("use only one of key, keys, key_env")
	}
	if len(c.ChannelKeys()) == 0 {
		if c.KeyEnv != "" {
			return fmt.Errorf("key is empty: environment variable %s is not set", c.KeyEnv)
		}
		return errors.New("key is required: set key, keys, or key_env")
	}

	baseURL := strings.TrimSpace(c.BaseURL)
	if baseURL == "" {
		if constant.GetChannelBaseURL(channelType) == "" {
			return fmt.Errorf("base_url is required for channel type %d (%s)", channelType, constant.GetChannelTypeName(channelType))
		}
	} else if err := validateBaseURL(baseURL); err != nil {
		return err
	}

	if c.Status != nil && *c.Status != 0 && *c.Status != 1 {
		return errors.New("status must be 0 (disabled) or 1 (enabled)")
	}
	if c.AutoBan != nil && *c.AutoBan != 0 && *c.AutoBan != 1 {
		return errors.New("auto_ban must be 0 or 1")
	}
	if c.Remark != nil && len(*c.Remark) > 255 {
		return errors.New("remark must not exceed 255 characters")
	}
	if c.MultiKeyMode != string(constant.MultiKeyModeRandom) && c.MultiKeyMode != string(constant.MultiKeyModePolling) {
		return fmt.Errorf("multi_key_mode must be %q or %q", constant.MultiKeyModeRandom, constant.MultiKeyModePolling)
	}

	if c.Models == nil || (!*c.Models.Fetch && len(c.Models.List) == 0) {
		return errors.New("models.list is required when models.fetch is false")
	}
	if _, err := compileGlobs(c.Models.Include); err != nil {
		return fmt.Errorf("models.include: %w", err)
	}
	if _, err := compileGlobs(c.Models.Exclude); err != nil {
		return fmt.Errorf("models.exclude: %w", err)
	}

	if err := validateOverrides(c.Overrides); err != nil {
		return fmt.Errorf("overrides: %w", err)
	}

	if c.Pricing != nil {
		if err := c.Pricing.validate(); err != nil {
			return fmt.Errorf("pricing: %w", err)
		}
	}
	return nil
}

// validateOverrides catches malformed operations before they reach the relay
// layer, where an unknown shape is silently treated as a plain path map and
// can inject an "operations" field into every upstream request.
func validateOverrides(overrides map[string]any) error {
	rawOperations, exists := overrides["operations"]
	if !exists {
		return nil
	}
	operations, ok := rawOperations.([]any)
	if !ok {
		return errors.New("operations must be a list")
	}
	for index, rawOperation := range operations {
		operation, ok := rawOperation.(map[string]any)
		if !ok {
			return fmt.Errorf("operations[%d] must be a mapping", index)
		}
		if path, _ := operation["path"].(string); strings.TrimSpace(path) == "" {
			return fmt.Errorf("operations[%d].path is required", index)
		}
		if mode, _ := operation["mode"].(string); strings.TrimSpace(mode) == "" {
			return fmt.Errorf("operations[%d].mode is required", index)
		}
		if rawLogic, hasLogic := operation["logic"]; hasLogic {
			logic, _ := rawLogic.(string)
			if !strings.EqualFold(strings.TrimSpace(logic), "AND") && !strings.EqualFold(strings.TrimSpace(logic), "OR") {
				return fmt.Errorf("operations[%d].logic must be AND or OR", index)
			}
		}
		rawConditions, hasConditions := operation["conditions"]
		if !hasConditions {
			continue
		}
		conditions, ok := rawConditions.([]any)
		if !ok {
			return fmt.Errorf("operations[%d].conditions must be a list", index)
		}
		for conditionIndex, rawCondition := range conditions {
			condition, ok := rawCondition.(map[string]any)
			if !ok {
				return fmt.Errorf("operations[%d].conditions[%d] must be a mapping", index, conditionIndex)
			}
			if path, _ := condition["path"].(string); strings.TrimSpace(path) == "" {
				return fmt.Errorf("operations[%d].conditions[%d].path is required", index, conditionIndex)
			}
			if mode, _ := condition["mode"].(string); strings.TrimSpace(mode) == "" {
				return fmt.Errorf("operations[%d].conditions[%d].mode is required", index, conditionIndex)
			}
		}
	}
	return nil
}

func validateBaseURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("base_url %q must be an absolute http(s) URL", raw)
	}
	return nil
}

func (p *Pricing) validate() error {
	if p.Defaults != nil {
		if _, err := p.Defaults.toModelPrice(); err != nil {
			return fmt.Errorf("defaults: %w", err)
		}
	}
	for _, name := range sortedKeys(p.Models) {
		if strings.TrimSpace(name) == "" {
			return errors.New("model name must not be empty")
		}
		if _, err := p.Models[name].toModelPrice(); err != nil {
			return fmt.Errorf("model %q: %w", name, err)
		}
	}
	return nil
}

// ResolveChannelType converts the config `type` value into a channel type ID.
// Numeric IDs and provider names/aliases are both accepted.
func ResolveChannelType(value any) (int, error) {
	switch typed := value.(type) {
	case nil:
		return 0, errors.New("type is required")
	case int:
		return checkChannelType(typed)
	case int8:
		return checkChannelType(int(typed))
	case int16:
		return checkChannelType(int(typed))
	case int32:
		return checkChannelType(int(typed))
	case int64:
		return checkChannelType(int(typed))
	case uint:
		return checkChannelType(int(typed))
	case uint8:
		return checkChannelType(int(typed))
	case uint16:
		return checkChannelType(int(typed))
	case uint32:
		return checkChannelType(int(typed))
	case uint64:
		return checkChannelType(int(typed))
	case float64:
		if typed != math.Trunc(typed) {
			return 0, fmt.Errorf("channel type %v must be an integer", typed)
		}
		return checkChannelType(int(typed))
	case string:
		trimmed := strings.TrimSpace(typed)
		// Names win over numbers so the "360" provider keeps working even
		// though its name parses as an integer.
		if channelType, ok := channelTypeByName[normalizeTypeName(trimmed)]; ok {
			return channelType, nil
		}
		if number, err := strconv.Atoi(trimmed); err == nil {
			return checkChannelType(number)
		}
		return 0, fmt.Errorf("unknown channel type %q; use a numeric type ID or a provider name such as %q, %q, %q, %q", typed, "openai", "anthropic", "gemini", "cubestack")
	default:
		return 0, fmt.Errorf("channel type %T is not supported", value)
	}
}

func checkChannelType(channelType int) (int, error) {
	// constant.ChannelTypeDummy is declared without an expression, so it
	// repeats ChannelTypeCubeStack instead of acting as a sentinel. Validate
	// against the registered name table instead of a numeric upper bound.
	if _, ok := constant.ChannelTypeNames[channelType]; !ok || channelType <= constant.ChannelTypeUnknown {
		return 0, fmt.Errorf("channel type %d is not a valid channel type ID", channelType)
	}
	return channelType, nil
}

var channelTypeAliases = map[string]int{
	"cubestack": constant.ChannelTypeCubeStack,
	"claude":    constant.ChannelTypeAnthropic,
	"google":    constant.ChannelTypeGemini,
	"vertex":    constant.ChannelTypeVertexAi,
	"bedrock":   constant.ChannelTypeAws,
	"codex":     constant.ChannelTypeCodex,
	"newapi":    constant.ChannelTypeNewAPI,
	"grok":      constant.ChannelTypeXai,
	"kimi":      constant.ChannelTypeMoonshot,
	"glm":       constant.ChannelTypeZhipu_v4,
	"ark":       constant.ChannelTypeVolcEngine,
	"doubao":    constant.ChannelTypeVolcEngine,
	"dashscope": constant.ChannelTypeAli,
	"qwen":      constant.ChannelTypeAli,
}

var channelTypeByName = buildChannelTypeIndex()

func buildChannelTypeIndex() map[string]int {
	index := make(map[string]int, len(constant.ChannelTypeNames)+len(channelTypeAliases))
	for channelType, name := range constant.ChannelTypeNames {
		if channelType <= constant.ChannelTypeUnknown {
			continue
		}
		index[normalizeTypeName(name)] = channelType
	}
	for alias, channelType := range channelTypeAliases {
		if _, exists := index[alias]; !exists {
			index[alias] = channelType
		}
	}
	return index
}

func normalizeTypeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var builder strings.Builder
	builder.Grow(len(name))
	for _, char := range name {
		switch char {
		case ' ', '-', '_':
			continue
		default:
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

// FilterModels merges fetched and explicitly listed models. Include/exclude
// globs only apply to fetched models; explicit entries are always kept.
func FilterModels(fetched []string, explicit []string, include []string, exclude []string) ([]string, error) {
	includePatterns, err := compileGlobs(include)
	if err != nil {
		return nil, fmt.Errorf("include: %w", err)
	}
	excludePatterns, err := compileGlobs(exclude)
	if err != nil {
		return nil, fmt.Errorf("exclude: %w", err)
	}

	seen := make(map[string]struct{}, len(fetched)+len(explicit))
	models := make([]string, 0, len(fetched)+len(explicit))
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, exists := seen[name]; exists {
			return
		}
		seen[name] = struct{}{}
		models = append(models, name)
	}

	for _, name := range fetched {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if len(includePatterns) > 0 && !matchesAny(includePatterns, trimmed) {
			continue
		}
		if matchesAny(excludePatterns, trimmed) {
			continue
		}
		add(trimmed)
	}
	for _, name := range explicit {
		add(name)
	}
	sort.Strings(models)
	return models, nil
}

// BuildPricePlan resolves the price of every model in the given set by
// applying layers in order, so later layers win. Models named by a layer but
// absent from the set are returned so callers can warn about dead config.
func BuildPricePlan(models []string, layers []PriceLayer) (map[string]ModelPrice, []string, error) {
	scope := make(map[string]struct{}, len(models))
	for _, name := range models {
		scope[name] = struct{}{}
	}
	plan := make(map[string]ModelPrice, len(models))
	unknown := make(map[string]struct{})

	for index, layer := range layers {
		if layer.Defaults != nil {
			price, err := layer.Defaults.toModelPrice()
			if err != nil {
				return nil, nil, fmt.Errorf("pricing layer %d defaults: %w", index, err)
			}
			for _, name := range models {
				plan[name] = price
			}
		}
		for _, name := range sortedKeys(layer.Models) {
			if _, ok := scope[name]; !ok {
				unknown[name] = struct{}{}
				continue
			}
			price, err := layer.Models[name].toModelPrice()
			if err != nil {
				return nil, nil, fmt.Errorf("pricing layer %d model %q: %w", index, name, err)
			}
			plan[name] = price
		}
	}
	return plan, sortedSet(unknown), nil
}

func (p Price) toModelPrice() (ModelPrice, error) {
	if p.PerRequest != nil {
		if p.Input != nil || p.Output != nil || p.CacheRead != nil || p.CacheWrite != nil {
			return ModelPrice{}, errors.New("per_request cannot be combined with token prices")
		}
		if err := requirePositiveFinite("per_request", *p.PerRequest); err != nil {
			return ModelPrice{}, err
		}
		perRequest := *p.PerRequest
		return ModelPrice{PerRequest: &perRequest}, nil
	}
	if p.Input == nil || p.Output == nil {
		return ModelPrice{}, errors.New("input and output prices are required (USD per 1M tokens)")
	}
	if err := requirePositiveFinite("input", *p.Input); err != nil {
		return ModelPrice{}, err
	}
	if err := requirePositiveFinite("output", *p.Output); err != nil {
		return ModelPrice{}, err
	}
	price := ModelPrice{
		ModelRatio:      *p.Input / 2,
		CompletionRatio: *p.Output / *p.Input,
	}
	if p.CacheRead != nil {
		if err := requireNonNegativeFinite("cache_read", *p.CacheRead); err != nil {
			return ModelPrice{}, err
		}
		cacheRatio := *p.CacheRead / *p.Input
		price.CacheRatio = &cacheRatio
	}
	if p.CacheWrite != nil {
		if err := requireNonNegativeFinite("cache_write", *p.CacheWrite); err != nil {
			return ModelPrice{}, err
		}
		createCacheRatio := *p.CacheWrite / *p.Input
		price.CreateCacheRatio = &createCacheRatio
	}
	return price, nil
}

// ToPrice converts the stored ratios back into USD prices per 1M tokens. It is
// the inverse of toModelPrice and is used for human-readable reporting.
func (p ModelPrice) ToPrice() Price {
	if p.PerRequest != nil {
		perRequest := *p.PerRequest
		return Price{PerRequest: &perRequest}
	}
	input := p.ModelRatio * 2
	output := input * p.CompletionRatio
	price := Price{Input: &input, Output: &output}
	if p.CacheRatio != nil {
		cacheRead := input * *p.CacheRatio
		price.CacheRead = &cacheRead
	}
	if p.CreateCacheRatio != nil {
		cacheWrite := input * *p.CreateCacheRatio
		price.CacheWrite = &cacheWrite
	}
	return price
}

func requirePositiveFinite(field string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return fmt.Errorf("%s must be a positive number", field)
	}
	return nil
}

func requireNonNegativeFinite(field string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return fmt.Errorf("%s must be a non-negative number", field)
	}
	return nil
}

func compileGlobs(patterns []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		trimmed := strings.TrimSpace(pattern)
		if trimmed == "" {
			return nil, errors.New("empty pattern")
		}
		var builder strings.Builder
		builder.WriteString("^")
		for _, char := range trimmed {
			switch char {
			case '*':
				builder.WriteString(".*")
			case '?':
				builder.WriteString(".")
			default:
				builder.WriteString(regexp.QuoteMeta(string(char)))
			}
		}
		builder.WriteString("$")
		compiledPattern, err := regexp.Compile(builder.String())
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		compiled = append(compiled, compiledPattern)
	}
	return compiled, nil
}

func matchesAny(patterns []*regexp.Regexp, value string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

func equalFloatPointer(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSet(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
