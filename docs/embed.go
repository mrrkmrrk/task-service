package docs

import "embed"

//go:embed index.html openapi.json swagger-ru.js swagger-ui.css swagger-ui-bundle.js swagger-ui-bundle.js.LICENSE.txt LICENSE NOTICE
var Files embed.FS
