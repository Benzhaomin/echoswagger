package echoswagger

import (
	_ "embed" // Enable the embed feature
)

const DefaultCDN = "https://cdn.jsdelivr.net/npm/swagger-ui-dist@4.6.2"

//go:embed "assets/swagger.html"
var swaggerHTMLTemplate string

//go:embed "assets/oauth2-redirect.html"
var oauthRedirect string
