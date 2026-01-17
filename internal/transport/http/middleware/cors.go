package middleware

import (
	"net/url"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/yeying-community/router/common/config"
)

func CORS() gin.HandlerFunc {
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowCredentials = true
	if len(config.CorsAllowedOrigins) == 0 {
		corsConfig.AllowOriginFunc = func(origin string) bool {
			return origin != ""
		}
	} else {
		allowed := make([]string, 0, len(config.CorsAllowedOrigins))
		allowAll := false
		for _, origin := range config.CorsAllowedOrigins {
			origin = strings.TrimSpace(origin)
			if origin == "" {
				continue
			}
			if origin == "*" {
				allowAll = true
				continue
			}
			allowed = append(allowed, origin)
		}
		corsConfig.AllowOriginFunc = func(origin string) bool {
			if origin == "" {
				return false
			}
			if allowAll {
				return true
			}
			parsed, err := url.Parse(origin)
			if err != nil {
				return false
			}
			for _, pattern := range allowed {
				if matchOrigin(parsed, pattern) {
					return true
				}
			}
			return false
		}
	}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{
		"Authorization",
		"Content-Type",
		"Accept",
		"Origin",
		"X-Requested-With",
		"X-Request-Id",
	}
	return cors.New(corsConfig)
}

func matchOrigin(origin *url.URL, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}

	schemePattern := ""
	hostPattern := pattern
	if strings.Contains(pattern, "://") {
		parts := strings.SplitN(pattern, "://", 2)
		schemePattern = strings.ToLower(parts[0])
		hostPattern = parts[1]
	}
	if idx := strings.Index(hostPattern, "/"); idx >= 0 {
		hostPattern = hostPattern[:idx]
	}

	originScheme := strings.ToLower(origin.Scheme)
	originHost := strings.ToLower(origin.Hostname())
	originPort := origin.Port()
	if originPort == "" {
		if originScheme == "https" {
			originPort = "443"
		} else if originScheme == "http" {
			originPort = "80"
		}
	}

	if schemePattern != "" && schemePattern != "*" && schemePattern != originScheme {
		return false
	}

	hostPattern = strings.ToLower(hostPattern)
	patternPort := ""
	if strings.HasPrefix(hostPattern, "[") {
		if idx := strings.LastIndex(hostPattern, "]"); idx >= 0 && len(hostPattern) > idx+1 && hostPattern[idx+1] == ':' {
			patternPort = hostPattern[idx+2:]
			hostPattern = hostPattern[:idx+1]
		}
	} else if idx := strings.LastIndex(hostPattern, ":"); idx > -1 {
		patternPort = hostPattern[idx+1:]
		hostPattern = hostPattern[:idx]
	}

	if patternPort != "" && originPort != "" && patternPort != originPort {
		return false
	}

	switch {
	case hostPattern == "*":
		return true
	case strings.HasPrefix(hostPattern, "*."):
		suffix := strings.TrimPrefix(hostPattern, "*.")
		if suffix == "" {
			return false
		}
		if !strings.HasSuffix(originHost, "."+suffix) {
			return false
		}
		return originHost != suffix
	default:
		return originHost == hostPattern
	}
}
