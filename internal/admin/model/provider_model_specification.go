package model

import (
	"encoding/json"
	"sort"
	"strings"
)

type ProviderModelSpecification struct {
	Version   int                                           `json:"version,omitempty"`
	Endpoints map[string]ProviderModelEndpointSpecification `json:"endpoints,omitempty"`
}

type ProviderModelEndpointSpecification struct {
	InputModalities  []string                                       `json:"input_modalities,omitempty"`
	OutputModalities []string                                       `json:"output_modalities,omitempty"`
	Parameters       map[string]ProviderModelParameterSpecification `json:"parameters,omitempty"`
	Constraints      *ProviderModelConstraintSpecification          `json:"constraints,omitempty"`
}

type ProviderModelParameterSpecification struct {
	Type          string   `json:"type,omitempty"`
	AllowedValues []string `json:"allowed_values,omitempty"`
	Min           *int     `json:"min,omitempty"`
	Max           *int     `json:"max,omitempty"`
}

type ProviderModelConstraintSpecification struct {
	MinPixels           *int     `json:"min_pixels,omitempty"`
	MaxPixels           *int     `json:"max_pixels,omitempty"`
	MinEdge             *int     `json:"min_edge,omitempty"`
	MaxEdge             *int     `json:"max_edge,omitempty"`
	EdgeMultiple        *int     `json:"edge_multiple,omitempty"`
	AllowedAspectRatios []string `json:"allowed_aspect_ratios,omitempty"`
}

func NormalizeProviderModelSpecification(spec *ProviderModelSpecification) *ProviderModelSpecification {
	if spec == nil {
		return nil
	}
	normalized := &ProviderModelSpecification{
		Version: spec.Version,
	}
	if normalized.Version < 0 {
		normalized.Version = 0
	}
	if len(spec.Endpoints) > 0 {
		normalized.Endpoints = make(map[string]ProviderModelEndpointSpecification, len(spec.Endpoints))
		for rawEndpoint, endpointSpec := range spec.Endpoints {
			endpoint := NormalizeRequestedChannelModelEndpoint(rawEndpoint)
			if endpoint == "" {
				endpoint = strings.TrimSpace(rawEndpoint)
			}
			if endpoint == "" {
				continue
			}
			normalizedEndpoint := ProviderModelEndpointSpecification{
				InputModalities:  normalizeSpecificationValues(endpointSpec.InputModalities),
				OutputModalities: normalizeSpecificationValues(endpointSpec.OutputModalities),
			}
			if len(endpointSpec.Parameters) > 0 {
				normalizedEndpoint.Parameters = make(map[string]ProviderModelParameterSpecification, len(endpointSpec.Parameters))
				for rawName, parameterSpec := range endpointSpec.Parameters {
					name := strings.TrimSpace(rawName)
					if name == "" {
						continue
					}
					next := ProviderModelParameterSpecification{
						Type:          strings.TrimSpace(strings.ToLower(parameterSpec.Type)),
						AllowedValues: normalizeSpecificationValues(parameterSpec.AllowedValues),
					}
					if parameterSpec.Min != nil {
						value := *parameterSpec.Min
						next.Min = &value
					}
					if parameterSpec.Max != nil {
						value := *parameterSpec.Max
						next.Max = &value
					}
					normalizedEndpoint.Parameters[name] = next
				}
			}
			if endpointSpec.Constraints != nil {
				constraints := &ProviderModelConstraintSpecification{
					AllowedAspectRatios: normalizeSpecificationValues(endpointSpec.Constraints.AllowedAspectRatios),
				}
				if endpointSpec.Constraints.MinPixels != nil {
					value := *endpointSpec.Constraints.MinPixels
					constraints.MinPixels = &value
				}
				if endpointSpec.Constraints.MaxPixels != nil {
					value := *endpointSpec.Constraints.MaxPixels
					constraints.MaxPixels = &value
				}
				if endpointSpec.Constraints.MinEdge != nil {
					value := *endpointSpec.Constraints.MinEdge
					constraints.MinEdge = &value
				}
				if endpointSpec.Constraints.MaxEdge != nil {
					value := *endpointSpec.Constraints.MaxEdge
					constraints.MaxEdge = &value
				}
				if endpointSpec.Constraints.EdgeMultiple != nil {
					value := *endpointSpec.Constraints.EdgeMultiple
					constraints.EdgeMultiple = &value
				}
				if constraints.MinPixels != nil ||
					constraints.MaxPixels != nil ||
					constraints.MinEdge != nil ||
					constraints.MaxEdge != nil ||
					constraints.EdgeMultiple != nil ||
					len(constraints.AllowedAspectRatios) > 0 {
					normalizedEndpoint.Constraints = constraints
				}
			}
			normalized.Endpoints[endpoint] = normalizedEndpoint
		}
	}
	if normalized.Version == 0 && len(normalized.Endpoints) == 0 {
		return nil
	}
	return normalized
}

func normalizeSpecificationValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	if len(result) == 0 {
		return nil
	}
	return result
}

func MarshalProviderModelSpecification(spec *ProviderModelSpecification) string {
	normalized := NormalizeProviderModelSpecification(spec)
	if normalized == nil {
		return ""
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	return string(payload)
}

func ParseProviderModelSpecification(raw string) (*ProviderModelSpecification, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	spec := ProviderModelSpecification{}
	if err := json.Unmarshal([]byte(trimmed), &spec); err != nil {
		return nil, err
	}
	return NormalizeProviderModelSpecification(&spec), nil
}
