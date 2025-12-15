package ui

import (
	"github.com/charmbracelet/lipgloss"
)

type Theme struct {
	Renderer *lipgloss.Renderer

	// Colors
	Primary   lipgloss.AdaptiveColor
	Secondary lipgloss.AdaptiveColor
	Subtext   lipgloss.AdaptiveColor

	// Status
	Open       lipgloss.AdaptiveColor
	InProgress lipgloss.AdaptiveColor
	Blocked    lipgloss.AdaptiveColor
	Closed     lipgloss.AdaptiveColor

	// Types
	Bug     lipgloss.AdaptiveColor
	Feature lipgloss.AdaptiveColor
	Task    lipgloss.AdaptiveColor
	Epic    lipgloss.AdaptiveColor
	Chore   lipgloss.AdaptiveColor

	// UI Elements
	Border    lipgloss.AdaptiveColor
	Highlight lipgloss.AdaptiveColor
	Muted     lipgloss.AdaptiveColor

	// Styles
	Base     lipgloss.Style
	Selected lipgloss.Style
	Column   lipgloss.Style
	Header   lipgloss.Style
}

// DefaultTheme returns the standard Dracula-inspired theme (adaptive)
func DefaultTheme(r *lipgloss.Renderer) Theme {
	t := Theme{
		Renderer: r,

		// Dracula / Light Mode equivalent
		Primary:   lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#BD93F9"}, // Purple
		Secondary: lipgloss.AdaptiveColor{Light: "#555555", Dark: "#6272A4"}, // Gray
		Subtext:   lipgloss.AdaptiveColor{Light: "#999999", Dark: "#BFBFBF"}, // Dim

		Open:       lipgloss.AdaptiveColor{Light: "#00A800", Dark: "#50FA7B"}, // Green
		InProgress: lipgloss.AdaptiveColor{Light: "#007EA8", Dark: "#8BE9FD"}, // Cyan
		Blocked:    lipgloss.AdaptiveColor{Light: "#D80000", Dark: "#FF5555"}, // Red
		Closed:     lipgloss.AdaptiveColor{Light: "#555555", Dark: "#6272A4"}, // Gray

		Bug:     lipgloss.AdaptiveColor{Light: "#D80000", Dark: "#FF5555"}, // Red
		Feature: lipgloss.AdaptiveColor{Light: "#D88000", Dark: "#FFB86C"}, // Orange
		Epic:    lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#BD93F9"}, // Purple
		Task:    lipgloss.AdaptiveColor{Light: "#A8A800", Dark: "#F1FA8C"}, // Yellow
		Chore:   lipgloss.AdaptiveColor{Light: "#007EA8", Dark: "#8BE9FD"}, // Cyan

		Border:    lipgloss.AdaptiveColor{Light: "#DDDDDD", Dark: "#44475A"},
		Highlight: lipgloss.AdaptiveColor{Light: "#EEEEEE", Dark: "#44475A"},
		Muted:     lipgloss.AdaptiveColor{Light: "#888888", Dark: "#6272A4"}, // Dimmed text
	}

	t.Base = r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#000000", Dark: "#F8F8F2"})

	t.Selected = r.NewStyle().
		Background(t.Highlight).
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(t.Primary).
		PaddingLeft(1).
		Bold(true)

	t.Header = r.NewStyle().
		Background(t.Primary).
		Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#282A36"}).
		Bold(true).
		Padding(0, 1)

	return t
}

func (t Theme) GetStatusColor(s string) lipgloss.AdaptiveColor {
	switch s {
	case "open":
		return t.Open
	case "in_progress":
		return t.InProgress
	case "blocked":
		return t.Blocked
	case "closed":
		return t.Closed
	default:
		return t.Subtext
	}
}

func (t Theme) GetTypeIcon(typ string) (string, lipgloss.AdaptiveColor) {
	switch typ {
	case "bug":
		return "🐛", t.Bug
	case "feature":
		return "✨", t.Feature
	case "task":
		return "📋", t.Task
	case "epic":
		// Use 🚀 instead of 🏔️ - the snow-capped mountain has a variation selector
		// (U+FE0F) that causes inconsistent width calculations across terminals
		return "🚀", t.Epic
	case "chore":
		return "🧹", t.Chore
	default:
		return "•", t.Subtext
	}
}
