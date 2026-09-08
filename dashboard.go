package main

import dashboardpkg "gryphdash/internal/dashboard"

type widgetCatalog = dashboardpkg.WidgetCatalog
type widget = dashboardpkg.Widget
type dashboard = dashboardpkg.Dashboard
type snapshot = dashboardpkg.Snapshot
type result = dashboardpkg.Result

var configuredWidgetCatalog = dashboardpkg.Catalog()

func buildDashboard(s snapshot) dashboard { return dashboardpkg.BuildDashboard(s) }
func object(v any) map[string]any         { return dashboardpkg.Object(v) }
func value(v any) string                  { return dashboardpkg.Value(v) }
func status(r result) string              { return dashboardpkg.Status(r) }
