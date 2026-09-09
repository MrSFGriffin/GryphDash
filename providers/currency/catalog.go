package currency

import "gryphdash/internal/dashboard"

func Catalog() dashboard.WidgetCatalog {
	return dashboard.WidgetCatalog{Widgets: []dashboard.WidgetConfig{
		{ID: "currency/eur-usd", Group: "Currency", Name: "EUR/USD exchange rate", Description: "US dollars per euro · Frankfurter reference rate", Width: 4, Height: 4, Logic: dashboard.WidgetLogic{Type: "scalar", Source: "currency/eur-usd", Path: "rate", URL: "https://api.frankfurter.dev/v2/rate/EUR/USD", Method: "GET"}},
		{ID: "currency/eur-gbp", Group: "Currency", Name: "EUR/GBP exchange rate", Description: "British pounds per euro · Frankfurter reference rate", Width: 4, Height: 4, Logic: dashboard.WidgetLogic{Type: "scalar", Source: "currency/eur-gbp", Path: "rate", URL: "https://api.frankfurter.dev/v2/rate/EUR/GBP", Method: "GET"}},
		{ID: "currency/eur-huf", Group: "Currency", Name: "EUR/HUF exchange rate", Description: "Hungarian forints per euro · Frankfurter reference rate", Width: 4, Height: 4, Logic: dashboard.WidgetLogic{Type: "scalar", Source: "currency/eur-huf", Path: "rate", URL: "https://api.frankfurter.dev/v2/rate/EUR/HUF", Method: "GET"}},
	}}
}
