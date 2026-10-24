package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

func (s *Server) openAPI(c *gin.Context) {
	path := filepath.Join("openapi.yaml")
	if _, err := os.Stat(path); err != nil {
		c.JSON(http.StatusOK, gin.H{"openapi": "3.0.0", "info": gin.H{"title": "Invoice Keeper API", "description": "Документация API", "version": "1.0.0"}})
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	var doc interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	out, err := json.Marshal(doc)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Data(http.StatusOK, "application/json", out)
}

func (s *Server) swaggerUI(c *gin.Context) {
	html := `<!DOCTYPE html>
<html lang="ru"><head><meta charset="utf-8"><title>Invoice Keeper — документация API</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"></head>
<body><div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
<script>
window.onload = function() {
  window.ui = SwaggerUIBundle({
    url: '/api/v1/openapi.json',
    dom_id: '#swagger-ui',
    deepLinking: true,
    presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
    layout: 'StandaloneLayout',
    defaultModelsExpandDepth: 1,
    displayRequestDuration: true,
    tryItOutEnabled: true,
    supportedSubmitMethods: ['get','post','put','patch','delete'],
    validatorUrl: null
  });
};
</script>
</body></html>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

