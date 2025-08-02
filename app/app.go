// Package app holds the definition of the web application.
package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/schema"
	"github.com/jspdown/traefik-playground/internal/compose"
	"github.com/jspdown/traefik-playground/internal/experiment"
	"github.com/rs/zerolog/log"
)

//go:embed templates/*
var templatesFS embed.FS

//go:embed dist/*
var assetsFS embed.FS

var schemaDecoder = schema.NewDecoder() //nolint:gochecknoglobals // Needed for caching.

// App is the web application.
type App struct {
	controller *experiment.Controller

	secretKey string

	assets fs.FS

	defaultDynamicConfig string

	experimentTemplate *template.Template
	infoTemplate       *template.Template
}

// New creates a new App.
func New(controller *experiment.Controller, secretKey string) (*App, error) {
	assets, err := fs.Sub(assetsFS, "dist")
	if err != nil {
		return nil, fmt.Errorf("accessing assets subtree: %w", err)
	}

	baseTemplate := template.Must(template.
		ParseFS(templatesFS, "templates/base.gohtml")).
		Funcs(template.FuncMap{
			"statusText": http.StatusText,
			"join":       strings.Join,
		})

	experimentTemplate := template.Must(template.Must(baseTemplate.Clone()).
		ParseFS(templatesFS, "templates/experiment.gohtml"))
	infoTemplate := template.Must(template.Must(baseTemplate.Clone()).
		ParseFS(templatesFS, "templates/info.gohtml"))

	defaultDynamicConfigFile, err := assets.Open("default-dynamic-configuration.yaml")
	if err != nil {
		return nil, fmt.Errorf("opening default dynamic configuration file: %w", err)
	}

	defaultDynamicConfig, err := io.ReadAll(defaultDynamicConfigFile)
	if err != nil {
		return nil, fmt.Errorf("reading default dynamic configuration file: %w", err)
	}

	return &App{
		controller:           controller,
		secretKey:            secretKey,
		assets:               assets,
		defaultDynamicConfig: string(defaultDynamicConfig),
		experimentTemplate:   experimentTemplate,
		infoTemplate:         infoTemplate,
	}, nil
}

// MountOn mounts the UI handler on the given muxer.
func (a *App) MountOn(mux *http.ServeMux) {
	mux.Handle("GET /", http.HandlerFunc(a.Experiment))
	mux.Handle("GET /info", http.HandlerFunc(a.Info))
	mux.Handle("POST /run", http.HandlerFunc(a.RunExperiment))
	mux.Handle("POST /share", http.HandlerFunc(a.ShareExperiment))
	mux.Handle("POST /export", http.HandlerFunc(a.ExportExperiment))
	mux.Handle("GET /share/{id}", http.HandlerFunc(a.SharedExperiment))

	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(a.assets))))
}

// Experiment serves the experiment page.
func (a *App) Experiment(rw http.ResponseWriter, req *http.Request) {
	a.render(req.Context(), rw, a.experimentTemplate, experimentTemplateData{
		DynamicConfig: a.defaultDynamicConfig,
	})
}

// Info serves the info page.
func (a *App) Info(rw http.ResponseWriter, req *http.Request) {
	a.render(req.Context(), rw, a.infoTemplate, nil)
}

type experimentTemplateData struct {
	DynamicConfig string
	Request       experimentTemplateRequestData
	Result        *experiment.Result

	RunBundle          string
	RunBundleSignature string

	ShareURL string

	Error error
}

type experimentTemplateRequestData struct {
	Method  string
	URL     string
	Headers string
	Body    string
}

func makeExperimentTemplateRequestData(req experiment.HTTPRequest) experimentTemplateRequestData {
	headers := make([]string, 0, len(req.Headers))
	for k := range req.Headers {
		headers = append(headers, k+": "+req.Headers.Get(k))
	}

	return experimentTemplateRequestData{
		Method:  req.Method,
		URL:     req.URL,
		Headers: strings.Join(headers, "\n"),
		Body:    req.Body,
	}
}

// RunExperiment runs an experiment.
func (a *App) RunExperiment(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var payload struct {
		DynamicConfig string `schema:"dynamicConfig"`
		Request       struct {
			Method  string `schema:"method"`
			URL     string `schema:"url"`
			Headers string `schema:"headers"`
			Body    string `schema:"body"`
		} `schema:"request"`
	}

	if err := decodeForm(req, &payload); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Failed read experiment")
		rw.WriteHeader(http.StatusBadRequest)

		a.render(req.Context(), rw, a.experimentTemplate, experimentTemplateData{
			DynamicConfig: a.defaultDynamicConfig,
			Error:         err,
		})

		return
	}

	exp, err := experiment.MakeExperiment(
		payload.DynamicConfig,
		payload.Request.Method,
		payload.Request.URL,
		payload.Request.Headers,
		payload.Request.Body)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Invalid experiment")
		rw.WriteHeader(http.StatusBadRequest)

		a.render(req.Context(), rw, a.experimentTemplate, experimentTemplateData{
			DynamicConfig: payload.DynamicConfig,
			Request:       experimentTemplateRequestData(payload.Request),
			Error:         err,
		})

		return
	}

	res, err := a.controller.Run(ctx, exp)
	if err != nil {
		log.Error().Err(err).Interface("experiment", exp).Msg("Unable to spawn experiment")

		if errors.Is(err, experiment.ErrRunTimeout) {
			rw.WriteHeader(http.StatusServiceUnavailable)
			err = errors.New("the service is currently busy, please retry later")
		} else {
			rw.WriteHeader(http.StatusInternalServerError)
			err = errors.New("the service is experiencing issues, please retry later")
		}

		a.render(req.Context(), rw, a.experimentTemplate, experimentTemplateData{
			DynamicConfig: exp.DynamicConfig,
			Request:       makeExperimentTemplateRequestData(exp.Request),
			Error:         err,
		})

		return
	}

	bundle, bundleSignature, err := marshalRunBundle(exp, res, a.secretKey)
	if err != nil {
		log.Error().Err(err).Interface("experiment", exp).Msg("Unable to marshal run bundle")
		rw.WriteHeader(http.StatusInternalServerError)

		a.render(req.Context(), rw, a.experimentTemplate, experimentTemplateData{
			DynamicConfig: exp.DynamicConfig,
			Request:       makeExperimentTemplateRequestData(exp.Request),
			Error:         errors.New("the service is experiencing issues, please retry later"),
		})

		return
	}

	a.render(req.Context(), rw, a.experimentTemplate, experimentTemplateData{
		DynamicConfig:      exp.DynamicConfig,
		Request:            makeExperimentTemplateRequestData(exp.Request),
		Result:             &res,
		RunBundle:          bundle,
		RunBundleSignature: bundleSignature,
	})
}

// ShareExperiment shares an experiment.
func (a *App) ShareExperiment(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var payload struct {
		RunBundle          string `schema:"runBundle"`
		RunBundleSignature string `schema:"runBundleSignature"`
	}

	if err := decodeForm(req, &payload); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Failed read experiment")
		rw.WriteHeader(http.StatusBadRequest)

		a.render(req.Context(), rw, a.experimentTemplate, experimentTemplateData{
			DynamicConfig: a.defaultDynamicConfig,
			Error:         err,
		})

		return
	}

	exp, res, err := unmarshalRunBundle(payload.RunBundle, payload.RunBundleSignature, a.secretKey)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Unable to unmarshal run bundle")
		rw.WriteHeader(http.StatusBadRequest)

		a.render(req.Context(), rw, a.experimentTemplate, experimentTemplateData{
			DynamicConfig: a.defaultDynamicConfig,
			Error:         err,
		})

		return
	}

	clientIP, _, _ := net.SplitHostPort(req.RemoteAddr)
	id, err := a.controller.Share(ctx, exp, res, clientIP)
	if err != nil {
		log.Error().Err(err).Interface("experiment", exp).Msg("Unable to share experiment")
		rw.WriteHeader(http.StatusInternalServerError)

		a.render(req.Context(), rw, a.experimentTemplate, experimentTemplateData{
			DynamicConfig: exp.DynamicConfig,
			Request:       makeExperimentTemplateRequestData(exp.Request),
			Result:        &res,
			Error:         errors.New("unable to share experiment, please retry later"),
		})

		return
	}

	http.Redirect(rw, req, "/share/"+id, http.StatusSeeOther)
}

// SharedExperiment serves a shared experiment.
func (a *App) SharedExperiment(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	id := req.PathValue("id")

	exp, res, err := a.controller.Shared(ctx, id)
	if err != nil {
		log.Error().Err(err).Str("id", id).Msg("Unable to retrieve experiment")

		if errors.Is(err, experiment.ErrNotFound) {
			rw.WriteHeader(http.StatusNotFound)
			err = errors.New("unable to find experiment")
		} else {
			rw.WriteHeader(http.StatusInternalServerError)
			err = errors.New("unable to retrieve experiment, please retry later")
		}

		a.render(req.Context(), rw, a.experimentTemplate, experimentTemplateData{
			DynamicConfig: a.defaultDynamicConfig,
			Error:         err,
		})

		return
	}

	bundle, bundleSignature, err := marshalRunBundle(exp, res, a.secretKey)
	if err != nil {
		log.Error().Err(err).Interface("experiment", exp).Msg("Unable to marshal run bundle")
		rw.WriteHeader(http.StatusInternalServerError)

		a.render(req.Context(), rw, a.experimentTemplate, experimentTemplateData{
			DynamicConfig: exp.DynamicConfig,
			Request:       makeExperimentTemplateRequestData(exp.Request),
			Error:         errors.New("the service is experiencing issues, please retry later"),
		})

		return
	}

	a.render(req.Context(), rw, a.experimentTemplate, experimentTemplateData{
		DynamicConfig:      exp.DynamicConfig,
		Request:            makeExperimentTemplateRequestData(exp.Request),
		Result:             &res,
		ShareURL:           req.URL.JoinPath(id).String(),
		RunBundle:          bundle,
		RunBundleSignature: bundleSignature,
	})
}

// ExportExperiment exports an experiment as a docker-compose file.
func (a *App) ExportExperiment(rw http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var payload struct {
		RunBundle          string `schema:"runBundle"`
		RunBundleSignature string `schema:"runBundleSignature"`
	}

	if err := decodeForm(req, &payload); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Failed to read export request")
		rw.WriteHeader(http.StatusBadRequest)

		return
	}

	exp, _, err := unmarshalRunBundle(payload.RunBundle, payload.RunBundleSignature, a.secretKey)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Unable to unmarshal run bundle")
		rw.WriteHeader(http.StatusBadRequest)

		return
	}

	dockerCompose := compose.Generate(exp.DynamicConfig)

	rw.Header().Set("Content-Type", "application/x-yaml")
	rw.Header().Set("Content-Disposition", `attachment; filename="docker-compose.yaml"`)
	rw.WriteHeader(http.StatusOK)

	if _, err = rw.Write([]byte(dockerCompose)); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Unable to write export response")
	}
}

type runBundle struct {
	Experiment experiment.Experiment `json:"experiment"`
	Result     experiment.Result     `json:"result"`
}

func marshalRunBundle(exp experiment.Experiment, res experiment.Result, secretKey string) (string, string, error) {
	marshaled, err := json.Marshal(runBundle{
		Experiment: exp,
		Result:     res,
	})
	if err != nil {
		return "", "", err
	}

	signature, err := generateHMAC(marshaled, secretKey)
	if err != nil {
		return "", "", fmt.Errorf("generating HMAC signature for run bundle: %w", err)
	}

	return base64.StdEncoding.EncodeToString(marshaled), signature, nil
}

func unmarshalRunBundle(bundle, signature, secretKey string) (exp experiment.Experiment, res experiment.Result, err error) {
	decoded, err := base64.StdEncoding.DecodeString(bundle)
	if err != nil {
		return
	}

	gotSignature, err := generateHMAC(decoded, secretKey)
	if err != nil {
		err = fmt.Errorf("generating HMAC signature from run bundle: %w", err)

		return
	}

	// Compare safely the received and compute signatures.
	if !hmac.Equal([]byte(gotSignature), []byte(signature)) {
		err = errors.New("invalid response signature")

		return
	}

	var b runBundle
	if err = json.Unmarshal(decoded, &b); err != nil {
		return
	}

	return b.Experiment, b.Result, nil
}

// generateHMAC creates an HMAC signature using SHA-256.
func generateHMAC(data []byte, secretKey string) (string, error) {
	h := hmac.New(sha256.New, []byte(secretKey))

	if _, err := h.Write(data); err != nil {
		return "", fmt.Errorf("writing data to HMAC: %w", err)
	}

	// Compute the HMAC digest and encode it in Base64.
	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func decodeForm(r *http.Request, v interface{}) error {
	if err := r.ParseForm(); err != nil {
		return err
	}

	return schemaDecoder.Decode(v, r.PostForm)
}

func (a *App) render(ctx context.Context, rw http.ResponseWriter, tmpl *template.Template, templateData any) {
	data := struct {
		Main any
	}{
		Main: templateData,
	}

	if err := tmpl.ExecuteTemplate(rw, "base", data); err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Unable to execute template")
		http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
