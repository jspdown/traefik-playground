package main

import (
	"os"
	"path"
	"reflect"

	"github.com/danielgtaylor/huma/v2"
	"github.com/ettle/strcase"
	"github.com/rs/zerolog/log"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"gopkg.in/yaml.v3"
	kyaml "sigs.k8s.io/yaml"
)

// Schema is a JSON schema.
type Schema struct {
	*huma.Schema `yaml:",inline"`

	ID          string        `yaml:"$id"`
	SchemaURL   string        `yaml:"$schema"`
	Definitions huma.Registry `yaml:"definitions"`
}

func main() {
	registry := huma.NewMapRegistry("#/definitions/", func(t reflect.Type, hint string) string {
		name := huma.DefaultSchemaNamer(t, hint)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}

		if t.PkgPath() == "" {
			return name
		}

		prefix := strcase.ToGoPascal(path.Base(t.PkgPath()))

		return prefix + name
	})

	schema := Schema{
		ID:          "https://traefik-playground.ozouf.fr/traefik-v3.schema.json",
		SchemaURL:   "http://json-schema.org/draft-07/schema#",
		Definitions: registry,
		Schema:      huma.SchemaFromType(registry, reflect.TypeOf(dynamic.Configuration{})),
	}

	cleanSchema(schema.Schema)
	for _, definition := range schema.Definitions.Map() {
		cleanSchema(definition)
	}

	schema.Title = "Traefik v3 Dynamic Configuration"

	yamlSchema, err := yaml.Marshal(schema)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to marshal JSON schema to YAML")
	}

	jsonSchema, err := kyaml.YAMLToJSONStrict(yamlSchema)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to convert YAML to JSON")
	}

	if _, err = os.Stdout.Write(jsonSchema); err != nil {
		log.Fatal().Err(err).Msg("Unable to write JSON schema on stdout")
	}
}

func cleanSchema(schema *huma.Schema) {
	// Huma adds an int32 and int64 format which is not part of the specification.
	if schema.Type == "integer" {
		schema.Format = ""
	}

	for _, subSchema := range schema.AllOf {
		cleanSchema(subSchema)
	}
	for _, subSchema := range schema.AnyOf {
		cleanSchema(subSchema)
	}
	for _, subSchema := range schema.OneOf {
		cleanSchema(subSchema)
	}

	if schema.Items != nil {
		cleanSchema(schema.Items)
	}
	if schema.Not != nil {
		cleanSchema(schema.Not)
	}

	for _, property := range schema.Properties {
		cleanSchema(property)
	}
}
