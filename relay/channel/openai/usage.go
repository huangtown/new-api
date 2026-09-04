package openai

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

// openAIErrorTypeToStatusCode maps OpenAI-compatible error type strings to
// their corresponding HTTP status codes. It is used to convert in-stream error
// events (which carry the semantic type from the upstream body rather than the
// HTTP status, because the stream was opened with 200 OK) into an appropriate
// client-facing status code so that, e.g., a rate-limit event surfaces as 429
// rather than 500.
func openAIErrorTypeToStatusCode(errType string) int {
	switch errType {
	case "invalid_request_error":
		return http.StatusBadRequest // 400
	case "authentication_error":
		return http.StatusUnauthorized // 401
	case "permission_error", "permission_denied_error":
		return http.StatusForbidden // 403
	case "not_found_error":
		return http.StatusNotFound // 404
	case "request_too_large":
		return http.StatusRequestEntityTooLarge // 413
	case "rate_limit_error":
		return http.StatusTooManyRequests // 429
	case "overloaded_error":
		return 529 // Anthropic-specific / equivalent to 503 in practice
	case "api_error", "server_error":
		return http.StatusInternalServerError // 500
	default:
		return http.StatusInternalServerError // 500 — unknown, preserve prior behaviour
	}
}

func applyUsagePostProcessing(info *relaycommon.RelayInfo, usage *dto.Usage, responseBody []byte) {
	if info == nil || usage == nil {
		return
	}

	switch info.ChannelType {
	case constant.ChannelTypeDeepSeek:
		if usage.PromptTokensDetails.CachedTokens == 0 && usage.PromptCacheHitTokens != 0 {
			usage.PromptTokensDetails.CachedTokens = usage.PromptCacheHitTokens
		}
	case constant.ChannelTypeZhipu_v4:
		// 智普的cached_tokens在标准位置: usage.prompt_tokens_details.cached_tokens
		if usage.PromptTokensDetails.CachedTokens == 0 {
			if usage.InputTokensDetails != nil && usage.InputTokensDetails.CachedTokens > 0 {
				usage.PromptTokensDetails.CachedTokens = usage.InputTokensDetails.CachedTokens
			} else if cachedTokens, ok := extractCachedTokensFromBody(responseBody); ok {
				usage.PromptTokensDetails.CachedTokens = cachedTokens
			} else if usage.PromptCacheHitTokens > 0 {
				usage.PromptTokensDetails.CachedTokens = usage.PromptCacheHitTokens
			}
		}
	case constant.ChannelTypeMoonshot:
		// Moonshot的cached_tokens在非标准位置: choices[].usage.cached_tokens
		if usage.PromptTokensDetails.CachedTokens == 0 {
			if usage.InputTokensDetails != nil && usage.InputTokensDetails.CachedTokens > 0 {
				usage.PromptTokensDetails.CachedTokens = usage.InputTokensDetails.CachedTokens
			} else if cachedTokens, ok := extractMoonshotCachedTokensFromBody(responseBody); ok {
				usage.PromptTokensDetails.CachedTokens = cachedTokens
			} else if cachedTokens, ok := extractCachedTokensFromBody(responseBody); ok {
				usage.PromptTokensDetails.CachedTokens = cachedTokens
			} else if usage.PromptCacheHitTokens > 0 {
				usage.PromptTokensDetails.CachedTokens = usage.PromptCacheHitTokens
			}
		}
	case constant.ChannelTypeOpenAI:
		if usage.PromptTokensDetails.CachedTokens == 0 {
			if cachedTokens, ok := extractLlamaCachedTokensFromBody(responseBody); ok {
				usage.PromptTokensDetails.CachedTokens = cachedTokens
			}
		}
	}

	// Apply the runtime cache read amplification ratio. The mutation lives here
	// (rather than in service.PostTextConsumeQuota) so that the response body
	// sent back to the caller also reflects the amplified value — the relay
	// adaptor marshals `usage` into the wire before billing is settled.
	//
	// OpenRouter is excluded because CalcOpenRouterCacheCreateTokens
	// (service/text_quota.go) reverse-derives cache creation tokens from the
	// upstream-reported cost using the ORIGINAL cache read token count;
	// feeding it the amplified value would skew the math. The billing path
	// for OpenRouter therefore sees the un-amplified value as well.
	if info.ChannelType != constant.ChannelTypeOpenRouter {
		amplifyCachedTokensForResponse(info, usage)
	}
}

// amplifyCachedTokensForResponse multiplies PromptTokensDetails.CachedTokens
// by the effective cache read amplification ratio for the relay's
// using-group when the ratio differs from 1.0 and the cached token count is
// non-zero. Exported as a package-level helper so the other OpenAI-family
// adaptors (Responses API, chat_via_responses, responses_via_chat) can call
// it directly.
//
// When info is nil (only the test helper does this), we fall back to the
// global common.CacheReadAmplificationRatio for symmetry with the per-group
// fallback path.
func amplifyCachedTokensForResponse(info *relaycommon.RelayInfo, usage *dto.Usage) {
	if usage == nil {
		return
	}
	var ratio float64
	if info != nil {
		ratio = ratio_setting.ResolveCacheReadAmplificationRatio(info.UsingGroup)
	} else {
		ratio = common.CacheReadAmplificationRatio
	}
	if ratio == 1.0 || usage.PromptTokensDetails.CachedTokens <= 0 {
		return
	}
	usage.PromptTokensDetails.CachedTokens = int(float64(usage.PromptTokensDetails.CachedTokens) * ratio)
}

func extractCachedTokensFromBody(body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}

	var payload struct {
		Usage struct {
			PromptTokensDetails struct {
				CachedTokens *int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			CachedTokens         *int `json:"cached_tokens"`
			PromptCacheHitTokens *int `json:"prompt_cache_hit_tokens"`
		} `json:"usage"`
	}

	if err := common.Unmarshal(body, &payload); err != nil {
		return 0, false
	}

	if payload.Usage.PromptTokensDetails.CachedTokens != nil {
		return *payload.Usage.PromptTokensDetails.CachedTokens, true
	}
	if payload.Usage.CachedTokens != nil {
		return *payload.Usage.CachedTokens, true
	}
	if payload.Usage.PromptCacheHitTokens != nil {
		return *payload.Usage.PromptCacheHitTokens, true
	}
	return 0, false
}

// extractMoonshotCachedTokensFromBody 从Moonshot的非标准位置提取cached_tokens
// Moonshot的流式响应格式: {"choices":[{"usage":{"cached_tokens":111}}]}
func extractMoonshotCachedTokensFromBody(body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}

	var payload struct {
		Choices []struct {
			Usage struct {
				CachedTokens *int `json:"cached_tokens"`
			} `json:"usage"`
		} `json:"choices"`
	}

	if err := common.Unmarshal(body, &payload); err != nil {
		return 0, false
	}

	// 遍历choices查找cached_tokens
	for _, choice := range payload.Choices {
		if choice.Usage.CachedTokens != nil && *choice.Usage.CachedTokens > 0 {
			return *choice.Usage.CachedTokens, true
		}
	}

	return 0, false
}

// extractLlamaCachedTokensFromBody 从llama.cpp的非标准位置提取cache_n
func extractLlamaCachedTokensFromBody(body []byte) (int, bool) {
	if len(body) == 0 {
		return 0, false
	}

	var payload struct {
		Timings struct {
			CachedTokens *int `json:"cache_n"`
		} `json:"timings"`
	}

	if err := common.Unmarshal(body, &payload); err != nil {
		return 0, false
	}

	if payload.Timings.CachedTokens == nil {
		return 0, false
	}
	return *payload.Timings.CachedTokens, true
}
