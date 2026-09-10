package openrouter

import "gryphdash/internal/dashboard"

func Catalog() dashboard.WidgetCatalog {
	return dashboard.WidgetCatalog{Widgets: []dashboard.WidgetConfig{
		{ID: "openrouter/spend", Group: "OpenRouter", Name: "Spend this month", Description: "Usage charged to the current OpenRouter API key this month · USD", Width: 4, Height: 4, Logic: dashboard.WidgetLogic{Type: "scalar", Source: "openrouter/key", Path: "usage_monthly", URL: "https://openrouter.ai/api/v1/key", Method: "GET"}},
		{ID: "openrouter/budget", Group: "OpenRouter", Name: "Budget remaining", Description: "Remaining configured limit for the current OpenRouter API key · USD", Width: 4, Height: 4, Logic: dashboard.WidgetLogic{Type: "scalar", Source: "openrouter/key", Path: "limit_remaining", URL: "https://openrouter.ai/api/v1/key", Method: "GET"}},
		{ID: "openrouter/credits", Group: "OpenRouter", Name: "Credit balance", Description: "Total purchased credits returned by OpenRouter · USD", Width: 4, Height: 4, Logic: dashboard.WidgetLogic{Type: "scalar", Source: "openrouter/credits", Path: "total_credits", URL: "https://openrouter.ai/api/v1/credits", Method: "GET"}},
		{ID: "openrouter/usage/daily", Group: "OpenRouter", Name: "Usage today", Description: "Usage charged to the current API key today · USD", Width: 4, Height: 4, Logic: dashboard.WidgetLogic{Type: "scalar", Source: "openrouter/key", Path: "usage_daily", URL: "https://openrouter.ai/api/v1/key", Method: "GET"}},
		{ID: "openrouter/usage/weekly", Group: "OpenRouter", Name: "Usage this week", Description: "Usage charged to the current API key this week · USD", Width: 4, Height: 4, Logic: dashboard.WidgetLogic{Type: "scalar", Source: "openrouter/key", Path: "usage_weekly", URL: "https://openrouter.ai/api/v1/key", Method: "GET"}},
		{ID: "openrouter/limit", Group: "OpenRouter", Name: "Key spending limit", Description: "Configured spending limit for the current OpenRouter API key · USD", Width: 4, Height: 4, Logic: dashboard.WidgetLogic{Type: "scalar", Source: "openrouter/key", Path: "limit", URL: "https://openrouter.ai/api/v1/key", Method: "GET"}},
		{ID: "openrouter/limit-reset", Group: "OpenRouter", Name: "Key limit reset", Description: "Reset period for the current API key", Width: 4, Height: 4, Logic: dashboard.WidgetLogic{Type: "scalar", Source: "openrouter/key", Path: "limit_reset", URL: "https://openrouter.ai/api/v1/key", Method: "GET"}},
		{ID: "openrouter/expires", Group: "OpenRouter", Name: "Key expiration", Description: "Expiration timestamp for the current API key", Width: 4, Height: 4, Logic: dashboard.WidgetLogic{Type: "timestamp", Source: "openrouter/key", Path: "expires_at", URL: "https://openrouter.ai/api/v1/key", Method: "GET"}},
		{ID: "openrouter/credits/used", Group: "OpenRouter", Name: "Credits used", Description: "Total credits used according to the management endpoint · USD", Width: 4, Height: 4, Logic: dashboard.WidgetLogic{Type: "scalar", Source: "openrouter/credits", Path: "total_usage", URL: "https://openrouter.ai/api/v1/credits", Method: "GET"}},
	}}
}
