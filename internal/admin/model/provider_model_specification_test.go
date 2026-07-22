package model

import "testing"

func TestProviderModelSpecificationSupportsFileInputRequiresTransportAndTypes(t *testing.T) {
	base := &ProviderModelSpecification{Endpoints: map[string]ProviderModelEndpointSpecification{
		"/v1/audio/transcriptions": {
			InputModalities: []string{"audio_file"},
			FileTypes:       []string{"mp3"},
			SupportsUpload:  true,
		},
	}}
	if !ProviderModelSpecificationSupportsFileInput(base) {
		t.Fatal("expected audio file specification to support file input")
	}
	base.Endpoints["/v1/audio/transcriptions"] = ProviderModelEndpointSpecification{InputModalities: []string{"audio_file"}, FileTypes: []string{"mp3"}}
	if ProviderModelSpecificationSupportsFileInput(base) {
		t.Fatal("file types without upload or URL transport must not qualify")
	}
}
