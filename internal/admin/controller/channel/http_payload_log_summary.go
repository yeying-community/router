package channel

import "github.com/yeying-community/router/common"

func encodeHTTPBodyForLog(body []byte, contentType string) map[string]any {
	return common.BuildPayloadLogFields(body, contentType)
}
