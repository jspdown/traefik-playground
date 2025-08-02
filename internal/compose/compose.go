// Package compose generates docker-compose files from experiments.
package compose

import (
	"fmt"
	"strings"
)

// Generate creates a docker-compose YAML configuration to test the given Traefik dynamic configuration.
func Generate(dynamicConfig string) string {
	dynamicConfig = transformDynamicConfigForDocker(dynamicConfig)

	return fmt.Sprintf(`configs:
  traefik-dynamic:
    content: |
%s

services:
  traefik:
    image: traefik:v3.4.4
    command:
      - --api.insecure=true
      - --providers.file.filename=/etc/traefik/dynamic.yaml
      - --providers.docker=true
      - --providers.docker.exposedByDefault=false
      - --entrypoints.web.address=:80
      - --log.level=debug
    ports:
      - "80:80"
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    configs:
      - source: traefik-dynamic
        target: /etc/traefik/dynamic.yaml
    networks:
      - traefik-network

  whoami:
    image: traefik/whoami
    networks:
      - traefik-network
    labels:
      - "traefik.enable=true"
      - "traefik.http.services.whoami.loadbalancer.server.port=80"

networks:
  traefik-network:
    driver: bridge
`, indentContent(dynamicConfig, "      "))
}

func indentContent(content, indent string) string {
	lines := strings.Split(content, "\n")
	indentedLines := make([]string, len(lines))

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			indentedLines[i] = ""
		} else {
			indentedLines[i] = indent + line
		}
	}

	return strings.Join(indentedLines, "\n")
}

func transformDynamicConfigForDocker(dynamicConfig string) string {
	// Replace the playground service references with docker container references
	// In the playground:
	//   - Services reference "http://10.10.10.10" for whoami (internal URL)
	//   - Service names use "whoami@playground" format
	// In docker-compose:
	//   - Services should reference "http://whoami:80" (container name:port)
	//   - Service names "whoami" comes from the docker provider.

	dynamicConfig = strings.ReplaceAll(dynamicConfig, "http://10.10.10.10", "http://whoami:80")
	dynamicConfig = strings.ReplaceAll(dynamicConfig, "whoami@playground", "whoami@docker")

	return dynamicConfig
}
