package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TutorialPage represents a single page of tutorial content.
type TutorialPage struct {
	ID       string   // Unique identifier (e.g., "intro", "navigation")
	Title    string   // Page title displayed in header
	Content  string   // Markdown content
	Section  string   // Parent section for TOC grouping
	Contexts []string // Which view contexts this page applies to (empty = all)
}

// TutorialModel manages the tutorial overlay state.
type TutorialModel struct {
	pages        []TutorialPage
	currentPage  int
	scrollOffset int
	tocVisible   bool
	progress     map[string]bool // Tracks which pages have been viewed
	width        int
	height       int
	theme        Theme
	contextMode  bool   // If true, filter pages by current context
	context      string // Current view context (e.g., "list", "board", "graph")

	// Markdown rendering with Glamour (bv-lb0h)
	markdownRenderer *MarkdownRenderer
}

// NewTutorialModel creates a new tutorial model with default pages.
func NewTutorialModel(theme Theme) TutorialModel {
	// Calculate initial content width for markdown renderer
	contentWidth := 80 - 6 // default width minus padding
	if contentWidth < 40 {
		contentWidth = 40
	}

	return TutorialModel{
		pages:            defaultTutorialPages(),
		currentPage:      0,
		scrollOffset:     0,
		tocVisible:       false,
		progress:         make(map[string]bool),
		width:            80,
		height:           24,
		theme:            theme,
		contextMode:      false,
		context:          "",
		markdownRenderer: NewMarkdownRendererWithTheme(contentWidth, theme),
	}
}

// Init initializes the tutorial model.
func (m TutorialModel) Init() tea.Cmd {
	return nil
}

// Update handles keyboard input for the tutorial.
func (m TutorialModel) Update(msg tea.Msg) (TutorialModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		// Navigation between pages
		case "right", "l", "n", "tab":
			m.NextPage()
		case "left", "h", "p", "shift+tab":
			m.PrevPage()

		// Scrolling within page
		case "j", "down":
			m.scrollOffset++
		case "k", "up":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case "g", "home":
			m.scrollOffset = 0
		case "G", "end":
			// Will be clamped in View()
			m.scrollOffset = 9999

		// TOC toggle
		case "t":
			m.tocVisible = !m.tocVisible

		// Jump to specific page (1-9)
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			pageNum := int(msg.String()[0] - '0')
			pages := m.visiblePages()
			if pageNum > 0 && pageNum <= len(pages) {
				m.JumpToPage(pageNum - 1)
			}
		}
	}
	return m, nil
}

// View renders the tutorial overlay.
func (m TutorialModel) View() string {
	pages := m.visiblePages()
	if len(pages) == 0 {
		return m.renderEmptyState()
	}

	// Clamp current page
	if m.currentPage >= len(pages) {
		m.currentPage = len(pages) - 1
	}
	if m.currentPage < 0 {
		m.currentPage = 0
	}

	currentPage := pages[m.currentPage]

	// Mark as viewed
	m.progress[currentPage.ID] = true

	r := m.theme.Renderer

	// Calculate dimensions
	contentWidth := m.width - 6 // padding and borders
	if m.tocVisible {
		contentWidth -= 24 // TOC sidebar width
	}
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Build the view
	var b strings.Builder

	// Header
	header := m.renderHeader(currentPage, len(pages))
	b.WriteString(header)
	b.WriteString("\n")

	// Separator line
	sepStyle := r.NewStyle().Foreground(m.theme.Border)
	b.WriteString(sepStyle.Render(strings.Repeat("─", contentWidth+4)))
	b.WriteString("\n")

	// Page title and section
	pageTitleStyle := r.NewStyle().Bold(true).Foreground(m.theme.Primary)
	sectionStyle := r.NewStyle().Foreground(m.theme.Subtext).Italic(true)
	pageTitle := pageTitleStyle.Render(currentPage.Title)
	if currentPage.Section != "" {
		pageTitle += sectionStyle.Render(" — " + currentPage.Section)
	}
	b.WriteString(pageTitle)
	b.WriteString("\n\n")

	// Content area (with optional TOC)
	if m.tocVisible {
		toc := m.renderTOC(pages)
		content := m.renderContent(currentPage, contentWidth)
		// Join TOC and content horizontally
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, toc, "  ", content))
	} else {
		content := m.renderContent(currentPage, contentWidth)
		b.WriteString(content)
	}

	b.WriteString("\n\n")

	// Footer with navigation hints
	footer := m.renderFooter(len(pages))
	b.WriteString(footer)

	// Wrap in modal style
	modalStyle := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary).
		Padding(1, 2).
		Width(m.width).
		MaxHeight(m.height)

	return modalStyle.Render(b.String())
}

// renderHeader renders the tutorial header with title and progress bar.
func (m TutorialModel) renderHeader(page TutorialPage, totalPages int) string {
	r := m.theme.Renderer

	titleStyle := r.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary)

	// Progress indicator: [2/15] ███░░░
	pageNum := m.currentPage + 1
	progressText := r.NewStyle().
		Foreground(m.theme.Subtext).
		Render(fmt.Sprintf("[%d/%d]", pageNum, totalPages))

	// Visual progress bar
	barWidth := 10
	filledWidth := 0
	if totalPages > 0 {
		filledWidth = (pageNum * barWidth) / totalPages
	}
	if filledWidth > barWidth {
		filledWidth = barWidth
	}
	progressBar := r.NewStyle().
		Foreground(m.theme.Open). // Using Open (green) for progress
		Render(strings.Repeat("█", filledWidth)) +
		r.NewStyle().
			Foreground(m.theme.Muted).
			Render(strings.Repeat("░", barWidth-filledWidth))

	// Title
	title := titleStyle.Render("📚 beads_viewer Tutorial")

	// Calculate spacing to align progress to the right
	headerContent := title + "  " + progressText + " " + progressBar

	return headerContent
}

// renderContent renders the page content with Glamour markdown and scroll handling.
func (m TutorialModel) renderContent(page TutorialPage, width int) string {
	r := m.theme.Renderer

	// Render markdown content using Glamour
	var renderedContent string
	if m.markdownRenderer != nil {
		rendered, err := m.markdownRenderer.Render(page.Content)
		if err == nil {
			renderedContent = strings.TrimSpace(rendered)
		} else {
			// Fallback to raw content on error
			renderedContent = page.Content
		}
	} else {
		renderedContent = page.Content
	}

	// Split rendered content into lines for scrolling
	lines := strings.Split(renderedContent, "\n")

	// Calculate visible lines based on height
	visibleHeight := m.height - 10 // header, footer, padding
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	// Clamp scroll offset
	maxScroll := len(lines) - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollOffset > maxScroll {
		m.scrollOffset = maxScroll
	}

	// Get visible lines
	endLine := m.scrollOffset + visibleHeight
	if endLine > len(lines) {
		endLine = len(lines)
	}
	visibleLines := lines[m.scrollOffset:endLine]

	// Join visible lines (already styled by Glamour)
	content := strings.Join(visibleLines, "\n")

	// Add scroll indicators
	if m.scrollOffset > 0 {
		scrollUpHint := r.NewStyle().Foreground(m.theme.Muted).Render("↑ more above")
		content = scrollUpHint + "\n" + content
	}
	if endLine < len(lines) {
		scrollDownHint := r.NewStyle().Foreground(m.theme.Muted).Render("↓ more below")
		content = content + "\n" + scrollDownHint
	}

	return content
}

// renderTOC renders the table of contents sidebar.
func (m TutorialModel) renderTOC(pages []TutorialPage) string {
	r := m.theme.Renderer

	tocStyle := r.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(m.theme.Border).
		Padding(0, 1).
		Width(22)

	headerStyle := r.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary)

	sectionStyle := r.NewStyle().
		Foreground(m.theme.Secondary).
		Bold(true)

	itemStyle := r.NewStyle().
		Foreground(m.theme.Subtext)

	selectedStyle := r.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary)

	viewedStyle := r.NewStyle().
		Foreground(m.theme.Open)

	var b strings.Builder
	b.WriteString(headerStyle.Render("Contents"))
	b.WriteString("\n")

	currentSection := ""
	for i, page := range pages {
		// Show section header if changed
		if page.Section != currentSection && page.Section != "" {
			currentSection = page.Section
			b.WriteString("\n")
			b.WriteString(sectionStyle.Render("▸ " + currentSection))
			b.WriteString("\n")
		}

		// Page entry with indentation under section
		prefix := "   "
		style := itemStyle
		if i == m.currentPage {
			prefix = " ▶ "
			style = selectedStyle
		}

		// Truncate long titles
		title := page.Title
		if len(title) > 14 {
			title = title[:12] + "…"
		}

		// Viewed indicator
		viewed := ""
		if m.progress[page.ID] {
			viewed = viewedStyle.Render(" ✓")
		}

		b.WriteString(style.Render(prefix+title) + viewed)
		b.WriteString("\n")
	}

	return tocStyle.Render(b.String())
}

// renderFooter renders navigation hints.
func (m TutorialModel) renderFooter(totalPages int) string {
	r := m.theme.Renderer

	keyStyle := r.NewStyle().
		Bold(true).
		Foreground(m.theme.Primary)

	descStyle := r.NewStyle().
		Foreground(m.theme.Subtext)

	sepStyle := r.NewStyle().
		Foreground(m.theme.Muted)

	// Build hints with styled keys
	hints := []string{
		keyStyle.Render("←/→") + descStyle.Render(" pages"),
		keyStyle.Render("j/k") + descStyle.Render(" scroll"),
		keyStyle.Render("t") + descStyle.Render(" TOC"),
		keyStyle.Render("1-9") + descStyle.Render(" jump"),
		keyStyle.Render("Esc") + descStyle.Render(" close"),
	}

	sep := sepStyle.Render(" │ ")
	return strings.Join(hints, sep)
}

// renderEmptyState renders a message when no pages are available.
func (m TutorialModel) renderEmptyState() string {
	r := m.theme.Renderer

	style := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.theme.Primary).
		Padding(2, 4).
		Width(m.width)

	return style.Render("No tutorial pages available for this context.")
}

// NextPage advances to the next page.
func (m *TutorialModel) NextPage() {
	pages := m.visiblePages()
	if m.currentPage < len(pages)-1 {
		m.currentPage++
		m.scrollOffset = 0
	}
}

// PrevPage goes to the previous page.
func (m *TutorialModel) PrevPage() {
	if m.currentPage > 0 {
		m.currentPage--
		m.scrollOffset = 0
	}
}

// JumpToPage jumps to a specific page index.
func (m *TutorialModel) JumpToPage(index int) {
	pages := m.visiblePages()
	if index >= 0 && index < len(pages) {
		m.currentPage = index
		m.scrollOffset = 0
	}
}

// JumpToSection jumps to the first page in a section.
func (m *TutorialModel) JumpToSection(sectionID string) {
	pages := m.visiblePages()
	for i, page := range pages {
		if page.ID == sectionID || page.Section == sectionID {
			m.currentPage = i
			m.scrollOffset = 0
			return
		}
	}
}

// SetContext sets the current view context for filtering.
func (m *TutorialModel) SetContext(ctx string) {
	m.context = ctx
	// Reset to first page when context changes
	m.currentPage = 0
	m.scrollOffset = 0
}

// SetContextMode enables or disables context-based filtering.
func (m *TutorialModel) SetContextMode(enabled bool) {
	m.contextMode = enabled
	if enabled {
		m.currentPage = 0
		m.scrollOffset = 0
	}
}

// SetSize sets the tutorial dimensions and updates the markdown renderer.
func (m *TutorialModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Update markdown renderer width to match content area
	contentWidth := width - 6 // padding and borders
	if m.tocVisible {
		contentWidth -= 24 // TOC sidebar width
	}
	if contentWidth < 40 {
		contentWidth = 40
	}

	if m.markdownRenderer != nil {
		m.markdownRenderer.SetWidthWithTheme(contentWidth, m.theme)
	}
}

// MarkViewed marks a page as viewed.
func (m *TutorialModel) MarkViewed(pageID string) {
	m.progress[pageID] = true
}

// Progress returns the progress map for persistence.
func (m TutorialModel) Progress() map[string]bool {
	return m.progress
}

// SetProgress restores progress from persistence.
func (m *TutorialModel) SetProgress(progress map[string]bool) {
	if progress != nil {
		m.progress = progress
	}
}

// CurrentPageID returns the ID of the current page.
func (m TutorialModel) CurrentPageID() string {
	pages := m.visiblePages()
	if m.currentPage >= 0 && m.currentPage < len(pages) {
		return pages[m.currentPage].ID
	}
	return ""
}

// IsComplete returns true if all pages have been viewed.
func (m TutorialModel) IsComplete() bool {
	pages := m.visiblePages()
	for _, page := range pages {
		if !m.progress[page.ID] {
			return false
		}
	}
	return len(pages) > 0
}

// visiblePages returns pages filtered by context if contextMode is enabled.
func (m TutorialModel) visiblePages() []TutorialPage {
	if !m.contextMode || m.context == "" {
		return m.pages
	}

	var filtered []TutorialPage
	for _, page := range m.pages {
		// Include if no context restriction or matches current context
		if len(page.Contexts) == 0 {
			filtered = append(filtered, page)
			continue
		}
		for _, ctx := range page.Contexts {
			if ctx == m.context {
				filtered = append(filtered, page)
				break
			}
		}
	}
	return filtered
}

// CenterTutorial returns the tutorial view centered in the terminal.
func (m TutorialModel) CenterTutorial(termWidth, termHeight int) string {
	tutorial := m.View()

	// Get actual rendered dimensions
	tutorialWidth := lipgloss.Width(tutorial)
	tutorialHeight := lipgloss.Height(tutorial)

	// Calculate padding
	padTop := (termHeight - tutorialHeight) / 2
	padLeft := (termWidth - tutorialWidth) / 2

	if padTop < 0 {
		padTop = 0
	}
	if padLeft < 0 {
		padLeft = 0
	}

	r := m.theme.Renderer

	centered := r.NewStyle().
		MarginTop(padTop).
		MarginLeft(padLeft).
		Render(tutorial)

	return centered
}

// defaultTutorialPages returns the built-in tutorial content.
// This is placeholder content - real content will come from bv-kdv2, bv-sbib, etc.
func defaultTutorialPages() []TutorialPage {
	return []TutorialPage{
		{
			ID:      "intro",
			Title:   "Welcome to bv",
			Section: "Getting Started",
			Content: `Welcome to **beads_viewer** (bv)!

bv is a powerful *TUI* (Terminal User Interface) for managing your project's issues using the **Beads** format.

This tutorial will guide you through:

- Navigating the interface
- Understanding views
- Working with beads (issues)
- Advanced features

> Press **→** or **n** to continue to the next page.`,
		},
		{
			ID:      "navigation",
			Title:   "Basic Navigation",
			Section: "Getting Started",
			Content: `## Navigation Basics

Use these keys to navigate:

| Key | Action |
|-----|--------|
| **j/k** or **↓/↑** | Move up/down in lists |
| **Enter** | Select/open item |
| **Esc** | Go back/close overlay |
| **q** | Quit bv |
| **?** | Show help overlay |

### Views

| Key | View |
|-----|------|
| **1** | List view (default) |
| **2** | Board view (Kanban) |
| **3** | Graph view (dependencies) |
| **4** | Labels view |
| **5** | History view |

> Press **→** to continue.`,
		},
		{
			ID:       "list-view",
			Title:    "List View",
			Section:  "Views",
			Contexts: []string{"list"},
			Content: `## List View

The **List view** shows all your beads in a filterable list.

### Filtering

| Key | Filter |
|-----|--------|
| **o** | Open issues only |
| **c** | Closed issues only |
| **r** | Ready issues (no blockers) |
| **a** | All issues |

### Sorting

- **s** - Cycle sort mode (priority, created, updated)
- **S** - Reverse sort order

### Search

Press **/** to start searching, then **n/N** for next/previous match.`,
		},
		{
			ID:       "board-view",
			Title:    "Board View",
			Section:  "Views",
			Contexts: []string{"board"},
			Content: `## Board View

The **Board view** shows a Kanban-style board with columns for each status:

1. **Open** - New issues
2. **In Progress** - Being worked on
3. **Blocked** - Waiting on dependencies
4. **Closed** - Completed

### Navigation

| Key | Action |
|-----|--------|
| **h/l** or **←/→** | Move between columns |
| **j/k** or **↓/↑** | Move within column |
| **Enter** | View issue details |
| **m** | Move issue to different status |`,
		},
		{
			ID:       "graph-view",
			Title:    "Graph View",
			Section:  "Views",
			Contexts: []string{"graph"},
			Content: `## Graph View

The **Graph view** visualizes dependencies between beads.

### Reading the Graph

- Arrow **→** points TO the dependency
- *Highlighted* node is currently selected

### Navigation

| Key | Action |
|-----|--------|
| **j/k** | Move between nodes |
| **Enter** | Select node |
| **f** | Focus on selected node's subgraph |`,
		},
		{
			ID:      "working-with-beads",
			Title:   "Working with Beads",
			Section: "Core Concepts",
			Content: `## Working with Beads

Each bead (issue) has:

- **ID** - Unique identifier (e.g., ` + "`bv-abc123`" + `)
- **Title** - Short description
- **Status** - open, in_progress, blocked, closed
- **Priority** - P0 (critical) to P4 (backlog)
- **Type** - bug, feature, task, epic, chore
- **Dependencies** - What it blocks/is blocked by

### Creating Beads

` + "```bash\nbd create --title=\"Fix login bug\" --type=bug --priority=1\n```" + `

### Updating Beads

` + "```bash\nbd update bv-abc123 --status=in_progress\n```",
		},
		{
			ID:      "ai-integration",
			Title:   "AI Agent Integration",
			Section: "Advanced",
			Content: `## AI Agent Integration

bv integrates seamlessly with **AI coding agents**.

### Robot Mode

` + "```bash\nbv --robot-triage   # Prioritized work recommendations\nbv --robot-next     # Single top priority item\nbv --robot-plan     # Parallel execution tracks\n```" + `

### AGENTS.md

The ` + "`AGENTS.md`" + ` file in your project helps AI agents understand your workflow and use bv effectively.

> See *AGENTS.md* for the complete AI integration guide.`,
		},
		{
			ID:      "keyboard-reference",
			Title:   "Keyboard Reference",
			Section: "Reference",
			Content: `## Quick Keyboard Reference

### Global

| Key | Action |
|-----|--------|
| **?** or **F1** | Help overlay |
| **q** | Quit |
| **Esc** | Close overlay / go back |
| **1-5** | Switch views |

### Navigation

| Key | Action |
|-----|--------|
| **j/k** | Move down/up |
| **h/l** | Move left/right |
| **g/G** | Go to top/bottom |
| **Enter** | Select |

### Filtering

| Key | Action |
|-----|--------|
| **/** | Search |
| **o/c/r/a** | Filter by status |

> For complete reference, press **?** in any view.`,
		},
	}
}
