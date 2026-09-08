package webassets

import "embed"

// FS contains the dashboard files shared by browser and desktop hosts.
//
//go:embed dashboard.html dashboard.js dashboard.css logo.svg vendor/gridstack/gridstack-all.js vendor/gridstack/gridstack.min.css
var FS embed.FS
