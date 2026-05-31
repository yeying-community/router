package model

import (
	"errors"
	"strings"
)

const (
	TopupErrorPaymentConfigMissing        = "payment_config_missing"
	TopupErrorPaymentConfigInvalid        = "payment_config_invalid"
	TopupErrorPaymentRequestBuildFailed   = "payment_request_build_failed"
	TopupErrorPaymentCreateFailed         = "payment_create_failed"
	TopupErrorPaymentCreateHTTPFailed     = "payment_create_http_failed"
	TopupErrorPaymentCreateUpstreamFailed = "payment_create_upstream_failed"
	TopupErrorPaymentQueryFailed          = "payment_query_failed"
	TopupErrorPaymentQueryHTTPFailed      = "payment_query_http_failed"
	TopupErrorPaymentQueryUpstreamFailed  = "payment_query_upstream_failed"
	TopupErrorPaymentResponseInvalid      = "payment_response_invalid"
	TopupErrorPaymentCallbackInvalid      = "payment_callback_invalid"
)

type TopupFlowError struct {
	Code    string
	Message string
	Err     error
}

func (err *TopupFlowError) Error() string {
	if err == nil {
		return ""
	}
	if strings.TrimSpace(err.Message) != "" {
		return strings.TrimSpace(err.Message)
	}
	if err.Err != nil {
		return err.Err.Error()
	}
	return strings.TrimSpace(err.Code)
}

func (err *TopupFlowError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Err
}

func NewTopupFlowError(code string, message string, err error) error {
	return &TopupFlowError{
		Code:    strings.TrimSpace(code),
		Message: strings.TrimSpace(message),
		Err:     err,
	}
}

func TopupErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var flowErr *TopupFlowError
	if errors.As(err, &flowErr) && flowErr != nil {
		return strings.TrimSpace(flowErr.Code)
	}
	return ""
}
