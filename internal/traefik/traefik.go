package traefik

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"

	"github.com/traefik/traefik/v3/cmd"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"github.com/traefik/traefik/v3/pkg/config/runtime"
	"github.com/traefik/traefik/v3/pkg/config/static"
	httpmuxer "github.com/traefik/traefik/v3/pkg/muxer/http"
	"github.com/traefik/traefik/v3/pkg/provider/aggregator"
	"github.com/traefik/traefik/v3/pkg/proxy/httputil"
	"github.com/traefik/traefik/v3/pkg/safe"
	"github.com/traefik/traefik/v3/pkg/server"
	"github.com/traefik/traefik/v3/pkg/server/middleware"
	"github.com/traefik/traefik/v3/pkg/server/router"
	"github.com/traefik/traefik/v3/pkg/server/service"
	"github.com/traefik/traefik/v3/pkg/tls"
)

const httpEntrypoint = "web"

// Traefik is a fake Traefik instance.
type Traefik struct {
	staticConfig  static.Configuration
	dynamicConfig *dynamic.Configuration

	handlerMu sync.RWMutex
	handlers  map[string]http.Handler

	readyFuncs []func()
}

// NewTraefik creates a new fake Traefik instance.
func NewTraefik(dynamicConfig *dynamic.Configuration) (*Traefik, error) {
	entryPoint := static.EntryPoint{Address: ":80"}
	entryPoint.SetDefaults()

	staticConfig := cmd.NewTraefikConfiguration().Configuration
	staticConfig.EntryPoints = map[string]*static.EntryPoint{
		httpEntrypoint: &entryPoint,
	}

	if err := staticConfig.ValidateConfiguration(); err != nil {
		return nil, fmt.Errorf("validating static configuration: %w", err)
	}

	return &Traefik{
		staticConfig:  staticConfig,
		dynamicConfig: dynamicConfig,
	}, nil
}

// OnReady registers a function to be called when the instance is ready to receive HTTP traffic.
func (t *Traefik) OnReady(readyFn func()) {
	t.readyFuncs = append(t.readyFuncs, readyFn)
}

// Start starts the Traefik instance.
func (t *Traefik) Start(ctx context.Context) error {
	whoami := NewWhoami()

	testServerInjector := NewServerInjector()
	testServerInjector.AddServer(Server{
		Name:       "whoami@playground",
		PublicURL:  "http://10.10.10.10",
		PrivateURL: whoami.URL,
	})

	parser, err := httpmuxer.NewSyntaxParser()
	if err != nil {
		return fmt.Errorf("creating syntax parser: %w", err)
	}

	providerAggregator := aggregator.NewProviderAggregator(static.Providers{})
	if err = providerAggregator.AddProvider(newProvider(t.dynamicConfig)); err != nil {
		return fmt.Errorf("adding file provider: %w", err)
	}

	pool := safe.NewPool(ctx)
	defaultEntryPoints := []string{httpEntrypoint}
	configWatcher := server.NewConfigurationWatcher(pool, providerAggregator, defaultEntryPoints, "file")

	// When the dynamic configuration changes, rebuild the handlers and notify the listeners.
	var firstConfigurationReceived bool
	configWatcher.AddListener(func(config dynamic.Configuration) {
		injectedDynamicConfig := testServerInjector.Inject(&config)
		handlers := buildHandlers(ctx, pool, parser, t.staticConfig, *injectedDynamicConfig)

		t.handlerMu.Lock()
		t.handlers = handlers
		if !firstConfigurationReceived {
			for _, readyFunc := range t.readyFuncs {
				readyFunc()
			}

			firstConfigurationReceived = true
		}
		t.handlerMu.Unlock()
	})

	configWatcher.Start()

	return nil
}

// Send sends an HTTP request to the fake Traefik instance.
func (t *Traefik) Send(req *http.Request) (*http.Response, error) {
	rw := httptest.NewRecorder()

	handler, ok := t.handlers[httpEntrypoint]
	if !ok {
		return nil, fmt.Errorf("no handler for entrypoint %q", httpEntrypoint)
	}

	handler.ServeHTTP(rw, req)

	return rw.Result(), nil
}

func buildHandlers(ctx context.Context, pool *safe.Pool, parser httpmuxer.SyntaxParser, staticConfig static.Configuration, dynamicConfig dynamic.Configuration) map[string]http.Handler {
	allEntryPointNames := slices.Collect(maps.Keys(staticConfig.EntryPoints))
	runtimeConfig := runtime.NewConfig(dynamicConfig)

	tlsManager := tls.NewManager()

	transportManager := service.NewTransportManager(nil)
	proxyBuilder := httputil.NewProxyBuilder(transportManager, nil)
	transportManager.Update(map[string]*dynamic.ServersTransport{
		"default@internal": {
			InsecureSkipVerify:  staticConfig.ServersTransport.InsecureSkipVerify,
			RootCAs:             staticConfig.ServersTransport.RootCAs,
			MaxIdleConnsPerHost: staticConfig.ServersTransport.MaxIdleConnsPerHost,
		},
	})

	serviceManager := service.NewManager(runtimeConfig.Services, nil, pool, transportManager, proxyBuilder)

	middlewaresBuilder := middleware.NewBuilder(runtimeConfig.Middlewares, serviceManager, nil)
	routerManager := router.NewManager(runtimeConfig, serviceManager, middlewaresBuilder, nil, tlsManager, parser)

	return routerManager.BuildHandlers(ctx, allEntryPointNames, false)
}

// ServerInjector injects Servers in the dynamic configuration.
type ServerInjector struct {
	testServers []Server
}

// NewServerInjector creates a new ServerInjector.
func NewServerInjector() *ServerInjector {
	return &ServerInjector{}
}

// Server holds the configuration of the server.
type Server struct {
	Name       string
	PublicURL  string
	PrivateURL string
}

// AddServer adds a new Server to inject.
func (i *ServerInjector) AddServer(server Server) {
	i.testServers = append(i.testServers, server)
}

// Inject injects the Servers in the given dynamic configuration.
func (i *ServerInjector) Inject(dynamicConfig *dynamic.Configuration) *dynamic.Configuration {
	dynamicConfig = dynamicConfig.DeepCopy()
	if dynamicConfig.HTTP.Services == nil {
		dynamicConfig.HTTP.Services = make(map[string]*dynamic.Service)
	}

	// Create the new HTTP services.
	for _, testServer := range i.testServers {
		dynamicConfig.HTTP.Services[testServer.Name] = &dynamic.Service{
			LoadBalancer: &dynamic.ServersLoadBalancer{
				Servers: []dynamic.Server{
					{URL: testServer.PrivateURL},
				},
			},
		}
	}

	// Replace the PublicURL with the PrivateURL in the services defined by the user.
	// This mechanism allows them to run experiment on HTTP service configuration options.
	for _, s := range dynamicConfig.HTTP.Services {
		if s.LoadBalancer == nil {
			continue
		}

		for serverIdx, server := range s.LoadBalancer.Servers {
			for _, testServer := range i.testServers {
				if server.URL == testServer.PublicURL {
					s.LoadBalancer.Servers[serverIdx].URL = testServer.PrivateURL
				}
			}
		}
	}

	return dynamicConfig
}
