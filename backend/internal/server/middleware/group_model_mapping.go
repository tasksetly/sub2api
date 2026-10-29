package middleware

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/gin-gonic/gin"
)

// GroupModelMapping rewrites public group model aliases to their upstream model
// before composite resolution, account scheduling, and protocol forwarding.
// Model allowlist validation is mounted before this middleware, so policy still
// evaluates the client-visible alias.
func GroupModelMapping() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey, ok := GetAPIKeyFromContext(c)
		if !ok || apiKey == nil || apiKey.Group == nil || apiKey.Group.Platform == service.PlatformComposite || len(apiKey.Group.ModelMapping) == 0 || c.Request == nil {
			c.Next()
			return
		}
		group := apiKey.Group

		// Model paths are metadata lookups and must retain the public alias so the
		// /models/:model response can be found in the alias-projected catalogue.
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodDelete {
			if model := strings.TrimSpace(c.Param("model")); model != "" && strings.Contains(c.FullPath(), "v1beta/models/:model") {
				if mapped, matched := group.ResolveGroupModel(model); matched && mapped != model {
					setGinParam(c, "model", mapped)
				}
			}
			if strings.TrimSpace(c.Query("model")) != "" {
				if mapped, matched := group.ResolveGroupModel(c.Query("model")); matched && mapped != c.Query("model") {
					q := c.Request.URL.Query()
					q.Set("model", mapped)
					c.Request.URL.RawQuery = q.Encode()
				}
			}
			c.Next()
			return
		}

		body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "Failed to read request body"}})
			c.Abort()
			return
		}
		mappedBody := rewriteGroupModelMappingBody(body, group)
		requestmodel.ResetRequestBody(c.Request, mappedBody)

		// Gemini's native endpoint embeds the model in the path segment
		// `models/{model}:{action}`. Rewrite it for the downstream handler while
		// keeping the public /models/:model metadata route unchanged above.
		if action := strings.TrimSpace(c.Param("modelAction")); action != "" {
			model, suffix := splitGeminiModelAction(action)
			if mapped, matched := group.ResolveGroupModel(model); matched && mapped != model {
				setGinParam(c, "modelAction", mapped+suffix)
			}
		}
		c.Next()
	}
}

func rewriteGroupModelMappingBody(body []byte, group *service.Group) []byte {
	if len(body) == 0 || group == nil {
		return body
	}
	out := body
	for _, path := range []string{"model", "session.model"} {
		result := gjson.GetBytes(out, path)
		if !result.Exists() || result.Type != gjson.String {
			continue
		}
		model := strings.TrimSpace(result.String())
		mapped, matched := group.ResolveGroupModel(model)
		if !matched || mapped == model {
			continue
		}
		updated, err := sjson.SetBytes(out, path, mapped)
		if err == nil {
			out = updated
		}
	}
	return out
}

func splitGeminiModelAction(value string) (string, string) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "/")
	if index := strings.LastIndex(value, ":"); index >= 0 {
		return strings.TrimSpace(value[:index]), value[index:]
	}
	return value, ""
}

func setGinParam(c *gin.Context, key, value string) {
	for i := range c.Params {
		if c.Params[i].Key == key {
			c.Params[i].Value = value
			return
		}
	}
	c.Params = append(c.Params, gin.Param{Key: key, Value: value})
}

