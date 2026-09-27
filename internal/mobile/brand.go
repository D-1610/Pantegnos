package mobile

import (
	"html/template"
	"strings"

	"Pantegnos/internal/brand"
)

// Logo is the Pantegnos mark, inlined from internal/brand so the interface, the
// web page, the launcher icon and the Windows binary all draw the same geometry.
//
// It is trusted markup: it comes from an embedded asset in this repository, not
// from user input, so it is deliberately not escaped again.
var Logo = template.HTML(strings.TrimSpace(brand.LogoSVG()))
