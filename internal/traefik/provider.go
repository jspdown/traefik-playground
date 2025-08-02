package traefik

import (
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"github.com/traefik/traefik/v3/pkg/safe"
)

// provider acts like if it was Traefik's file provider.
// We are reusing the name "file" provider because it use the same syntax as the file provider.
// It provides a single dynamic configuration.
type provider struct {
	config *dynamic.Configuration
}

// newProvider creates a new provider.
func newProvider(config *dynamic.Configuration) *provider {
	return &provider{
		config: config,
	}
}

// Init does nothing.
func (f *provider) Init() error { return nil }

// Provide provides the dynamic configuration.
func (f *provider) Provide(configurationChan chan<- dynamic.Message, _ *safe.Pool) error {
	configurationChan <- dynamic.Message{
		ProviderName:  "file",
		Configuration: f.config,
	}

	return nil
}
