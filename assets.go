// Package physiobase bettet Templates und statische Dateien in das Binary ein,
// damit zur Laufzeit kein Quellbaum gebraucht wird.
package physiobase

import "embed"

//go:embed web/templates/*.html
var TemplatesFS embed.FS

//go:embed all:web/static
var StaticFS embed.FS
