package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tuiTickMsg struct{}
type tuiLayout struct {
	Widgets []string `json:"widgets"`
}
type tuiModel struct {
	collector            *collector
	dashboard, available dashboard
	width, focus         int
	selected             []string
	picker               bool
	ready                bool
}

func runTUI(ctx context.Context) error {
	interval, err := refreshInterval()
	if err != nil {
		return err
	}
	c := newCollector()
	go c.run(ctx, interval)
	m := tuiModel{collector: c, selected: loadTUILayout()}
	_, err = tea.NewProgram(m, tea.WithContext(ctx), tea.WithAltScreen()).Run()
	return err
}
func (m tuiModel) Init() tea.Cmd { return tuiTick() }
func tuiTick() tea.Cmd           { return tea.Tick(time.Second, func(time.Time) tea.Msg { return tuiTickMsg{} }) }
func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.picker {
			switch msg.String() {
			case "esc", "a", "q":
				m.picker = false
			case "up", "k":
				if m.focus > 0 {
					m.focus--
				}
			case "down", "j":
				if m.focus < len(m.available.Widgets)-1 {
					m.focus++
				}
			case "enter", " ":
				if len(m.available.Widgets) > 0 {
					m.toggle(m.available.Widgets[m.focus].ID)
				}
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "a":
			m.picker = true
			m.focus = 0
		case "d", "x", "delete":
			m.remove()
		case "left", "h":
			m.navigate(-1, 0)
		case "right", "l":
			m.navigate(1, 0)
		case "shift+left", "shift+h":
			m.move(-1)
		case "shift+right", "shift+l":
			m.move(1)
		case "up", "k":
			m.navigate(0, -1)
		case "down", "j":
			m.navigate(0, 1)
		case "r":
			m.defaults()
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tuiTickMsg:
		snapshot := m.collector.snapshot()
		if !snapshotReady(snapshot) {
			return m, tuiTick()
		}
		m.available = buildDashboard(snapshot)
		if !hasLimitBuckets(snapshot) {
			filtered := m.available.Widgets[:0]
			for _, w := range m.available.Widgets {
				if !strings.HasPrefix(w.ID, "codex/bucket/") {
					filtered = append(filtered, w)
				}
			}
			m.available.Widgets = filtered
		}
		m.ready = true
		if m.selected == nil {
			for _, w := range m.available.Widgets {
				if w.Default {
					m.selected = append(m.selected, w.ID)
				}
			}
			m.save()
		}
		m.dashboard = m.apply(m.available)
		return m, tuiTick()
	}
	return m, nil
}

func hasLimitBuckets(s snapshot) bool {
	return len(object(s.Limits.Data["rateLimitsByLimitId"])) > 0 || len(object(s.Limits.Data["rateLimits"])) > 0
}

func snapshotReady(s snapshot) bool {
	for _, r := range []result{s.Account, s.Limits, s.Usage, s.OpenRouterKey, s.OpenRouterCredits} {
		if !r.Updated.IsZero() || r.Error != "" {
			return true
		}
	}
	return false
}

func (m *tuiModel) navigate(dx, dy int) {
	if m.focus < 0 || m.focus >= len(m.dashboard.Widgets) {
		return
	}
	positions := m.positions()
	from := positions[m.dashboard.Widgets[m.focus].ID]
	best, found := m.focus, false
	bestDistance := int(^uint(0) >> 1)
	for i, w := range m.dashboard.Widgets {
		p, ok := positions[w.ID]
		if !ok || i == m.focus {
			continue
		}
		if dy != 0 {
			if (dy > 0 && p.y <= from.y) || (dy < 0 && p.y >= from.y) {
				continue
			}
			distance := abs(p.y-from.y)*100 + abs(p.x-from.x)
			if distance < bestDistance {
				best, bestDistance, found = i, distance, true
			}
		} else if dx != 0 && p.y == from.y {
			if (dx > 0 && p.x <= from.x) || (dx < 0 && p.x >= from.x) {
				continue
			}
			distance := abs(p.x - from.x)
			if distance < bestDistance {
				best, bestDistance, found = i, distance, true
			}
		}
	}
	if found {
		m.focus = best
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

type tuiPosition struct{ x, y int }

func (m tuiModel) positions() map[string]tuiPosition {
	groups := map[string][]widget{}
	for _, w := range m.dashboard.Widgets {
		groups[w.Group] = append(groups[w.Group], w)
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	columns := 2
	if m.width > 0 && m.width < 90 {
		columns = 1
	}
	positions := make(map[string]tuiPosition, len(m.dashboard.Widgets))
	row := 0
	for _, group := range keys {
		for i, w := range groups[group] {
			positions[w.ID] = tuiPosition{x: i % columns, y: row + i/columns}
		}
		row += (len(groups[group])+columns-1)/columns + 1
	}
	return positions
}
func (m *tuiModel) toggle(id string) {
	for i, v := range m.selected {
		if v == id {
			m.selected = append(m.selected[:i], m.selected[i+1:]...)
			m.save()
			m.dashboard = m.apply(m.available)
			return
		}
	}
	m.selected = append(m.selected, id)
	m.save()
	m.dashboard = m.apply(m.available)
}
func (m *tuiModel) remove() {
	if m.focus < len(m.dashboard.Widgets) {
		m.toggle(m.dashboard.Widgets[m.focus].ID)
		if m.focus >= len(m.selected) {
			m.focus = len(m.selected) - 1
		}
		if m.focus < 0 {
			m.focus = 0
		}
		m.dashboard = m.apply(m.available)
	}
}
func (m *tuiModel) move(delta int) {
	j := m.focus + delta
	if m.focus < 0 || j < 0 || m.focus >= len(m.selected) || j >= len(m.selected) {
		return
	}
	m.selected[m.focus], m.selected[j] = m.selected[j], m.selected[m.focus]
	m.focus = j
	m.save()
	m.dashboard = m.apply(m.available)
}
func (m *tuiModel) defaults() {
	m.selected = nil
	for _, w := range buildDashboard(m.collector.snapshot()).Widgets {
		if w.Default {
			m.selected = append(m.selected, w.ID)
		}
	}
	m.focus = 0
	m.save()
	m.available = buildDashboard(m.collector.snapshot())
	m.dashboard = m.apply(m.available)
}
func (m tuiModel) apply(d dashboard) dashboard {
	if m.selected == nil {
		return d
	}
	all := map[string]widget{}
	for _, w := range d.Widgets {
		all[w.ID] = w
	}
	out := make([]widget, 0, len(m.selected))
	for _, id := range m.selected {
		if w, ok := all[id]; ok {
			out = append(out, w)
		}
	}
	d.Widgets = out
	return d
}
func tuiLayoutPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "gryphdash", "layout.json")
}
func loadTUILayout() []string {
	p := tuiLayoutPath()
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var l tuiLayout
	if json.Unmarshal(data, &l) != nil {
		return nil
	}
	return l.Widgets
}
func (m tuiModel) save() {
	p := tuiLayoutPath()
	if p == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0700)
	data, _ := json.MarshalIndent(tuiLayout{m.selected}, "", "  ")
	_ = os.WriteFile(p, data, 0600)
}

var (
	tuiTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	tuiGroup = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#04B575"))
	tuiCard  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#444444")).Padding(0, 1)
	tuiFocus = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#FFFFFF")).Padding(0, 1)
	tuiDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("#777777"))
)

func (m tuiModel) View() string {
	if m.picker {
		return m.pickerView()
	}
	groups := map[string][]widget{}
	for _, w := range m.dashboard.Widgets {
		groups[w.Group] = append(groups[w.Group], w)
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	controls := "a add • d delete • arrows move/focus • r defaults • q quit"
	if m.picker {
		controls = "Add widget: ↑/↓ choose • Enter toggle • Esc close"
	}
	var b strings.Builder
	b.WriteString(tuiTitle.Render("GryphDash"))
	b.WriteString("  ")
	b.WriteString(tuiDim.Render(controls + " • " + time.Now().Format("15:04:05 UTC")))
	b.WriteString("\n\n")
	if !m.ready {
		b.WriteString(tuiDim.Render("Loading provider metrics…"))
		return b.String()
	}
	if len(m.dashboard.Widgets) == 0 {
		b.WriteString(tuiDim.Render("No widgets selected. Press a to add one."))
		return b.String()
	}
	for _, group := range keys {
		b.WriteString(tuiGroup.Render(group))
		b.WriteByte('\n')
		cards := make([]string, 0, len(groups[group]))
		for _, w := range groups[group] {
			card := tuiCard.Render(renderTUICard(w))
			if m.focus < len(m.dashboard.Widgets) && m.dashboard.Widgets[m.focus].ID == w.ID {
				card = tuiFocus.Render(renderTUICard(w))
			}
			cards = append(cards, card)
		}
		columns := 2
		if m.width > 0 && m.width < 90 {
			columns = 1
		}
		for i := 0; i < len(cards); i += columns {
			end := i + columns
			if end > len(cards) {
				end = len(cards)
			}
			b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cards[i:end]...))
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	return b.String()
}
func (m tuiModel) pickerView() string {
	var b strings.Builder
	b.WriteString(tuiTitle.Render("Add or remove widgets"))
	b.WriteString("  ")
	b.WriteString(tuiDim.Render("↑/↓ choose • Enter toggle • Esc close"))
	b.WriteString("\n\n")
	for i, w := range m.available.Widgets {
		marker := "  "
		for _, id := range m.selected {
			if id == w.ID {
				marker = "✓ "
			}
		}
		if i == m.focus {
			marker = ">" + marker[1:]
		}
		fmt.Fprintf(&b, "%s%-30s %s\n", marker, w.Title, tuiDim.Render(w.Group))
	}
	return b.String()
}
func renderTUICard(w widget) string {
	value := w.Value
	if value == "" {
		value = "Unavailable"
	}
	status := w.Status
	if status == "" {
		status = "ok"
	}
	text := fmt.Sprintf("%s\n%s\n%s", w.Title, value, tuiDim.Render(status))
	if w.Note != "" {
		text += "\n" + tuiDim.Render(w.Note)
	}
	return text
}
