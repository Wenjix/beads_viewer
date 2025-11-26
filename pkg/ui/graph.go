package ui

import (
	"fmt"
	"sort"
	"strings"

	"beads_viewer/pkg/analysis"
	"beads_viewer/pkg/model"

	"github.com/charmbracelet/lipgloss"
)

// GraphModel represents the dependency graph view
type GraphModel struct {
	issues       []model.Issue
	issueMap     map[string]*model.Issue
	insights     *analysis.Insights
	selectedNode int
	scrollX      int
	scrollY      int
	width        int
	height       int
	theme        Theme
	nodes        []graphNode
	edges        []graphEdge
	layers       [][]int // Node indices per layer
	canvas       *canvas
}

// graphNode represents a node in the graph layout
type graphNode struct {
	id     string
	title  string
	x, y   int // Position in grid coordinates
	width  int // Width in characters
	height int // Height in characters
	layer  int
	status model.Status
	itype  model.IssueType
}

// graphEdge represents an edge between nodes
type graphEdge struct {
	from, to int  // Node indices
	depType  string
}

// canvas is a 2D character buffer for drawing
type canvas struct {
	cells  [][]rune
	colors [][]lipgloss.Style
	width  int
	height int
}

func newCanvas(width, height int) *canvas {
	cells := make([][]rune, height)
	colors := make([][]lipgloss.Style, height)
	for i := range cells {
		cells[i] = make([]rune, width)
		colors[i] = make([]lipgloss.Style, width)
		for j := range cells[i] {
			cells[i][j] = ' '
		}
	}
	return &canvas{cells: cells, colors: colors, width: width, height: height}
}

func (c *canvas) set(x, y int, r rune, style lipgloss.Style) {
	if x >= 0 && x < c.width && y >= 0 && y < c.height {
		c.cells[y][x] = r
		c.colors[y][x] = style
	}
}

func (c *canvas) get(x, y int) rune {
	if x >= 0 && x < c.width && y >= 0 && y < c.height {
		return c.cells[y][x]
	}
	return ' '
}

func (c *canvas) drawString(x, y int, s string, style lipgloss.Style) {
	for i, r := range []rune(s) {
		c.set(x+i, y, r, style)
	}
}

func (c *canvas) render(theme Theme, scrollX, scrollY, viewWidth, viewHeight int) string {
	var lines []string
	for y := scrollY; y < scrollY+viewHeight && y < c.height; y++ {
		var line strings.Builder
		for x := scrollX; x < scrollX+viewWidth && x < c.width; x++ {
			r := c.cells[y][x]
			style := c.colors[y][x]
			if style.GetForeground() == (lipgloss.NoColor{}) {
				line.WriteRune(r)
			} else {
				line.WriteString(style.Render(string(r)))
			}
		}
		lines = append(lines, line.String())
	}
	return strings.Join(lines, "\n")
}

// NewGraphModel creates a new graph view from issues
func NewGraphModel(issues []model.Issue, insights *analysis.Insights, theme Theme) GraphModel {
	issueMap := make(map[string]*model.Issue)
	for i := range issues {
		issueMap[issues[i].ID] = &issues[i]
	}

	g := GraphModel{
		issues:   issues,
		issueMap: issueMap,
		insights: insights,
		theme:    theme,
	}

	g.buildGraph()
	return g
}

// SetIssues updates the graph data
func (g *GraphModel) SetIssues(issues []model.Issue, insights *analysis.Insights) {
	g.issues = issues
	g.issueMap = make(map[string]*model.Issue)
	for i := range issues {
		g.issueMap[issues[i].ID] = &issues[i]
	}
	g.insights = insights
	g.buildGraph()
}

// buildGraph creates the graph layout
func (g *GraphModel) buildGraph() {
	// Clear existing graph data
	g.nodes = nil
	g.edges = nil
	g.layers = nil

	if len(g.issues) == 0 {
		return
	}

	// Build adjacency list (who depends on whom)
	dependsOn := make(map[string][]string) // A depends on B
	allIDs := make(map[string]bool)

	for _, issue := range g.issues {
		allIDs[issue.ID] = true
		for _, dep := range issue.Dependencies {
			if dep.Type == model.DepBlocks || dep.Type == model.DepParentChild {
				dependsOn[issue.ID] = append(dependsOn[issue.ID], dep.DependsOnID)
			}
		}
	}

	// Compute layers using longest path from roots
	layers := make(map[string]int)
	var computeLayer func(id string, visited map[string]bool) int
	computeLayer = func(id string, visited map[string]bool) int {
		if layer, ok := layers[id]; ok {
			return layer
		}
		if visited[id] {
			return 0 // Cycle detected
		}
		visited[id] = true
		maxParent := -1
		for _, parent := range dependsOn[id] {
			if _, exists := g.issueMap[parent]; exists {
				pl := computeLayer(parent, visited)
				if pl > maxParent {
					maxParent = pl
				}
			}
		}
		layers[id] = maxParent + 1
		return layers[id]
	}

	for id := range allIDs {
		computeLayer(id, make(map[string]bool))
	}

	// Group nodes by layer
	layerGroups := make(map[int][]string)
	maxLayer := 0
	for id, layer := range layers {
		layerGroups[layer] = append(layerGroups[layer], id)
		if layer > maxLayer {
			maxLayer = layer
		}
	}

	// Sort nodes within layers for consistency
	for layer := range layerGroups {
		sort.Strings(layerGroups[layer])
	}

	// Create nodes with positions
	g.nodes = nil
	g.edges = nil
	g.layers = make([][]int, maxLayer+1)

	nodeWidth := 20
	nodeHeight := 4 // 4 lines: top border, ID line, title line, bottom border
	horizontalGap := 4
	verticalGap := 2

	nodeIndexMap := make(map[string]int)

	for layer := 0; layer <= maxLayer; layer++ {
		ids := layerGroups[layer]
		g.layers[layer] = make([]int, len(ids))

		for i, id := range ids {
			issue := g.issueMap[id]
			if issue == nil {
				continue
			}

			x := i * (nodeWidth + horizontalGap)
			y := layer * (nodeHeight + verticalGap)

			title := truncateRunesHelper(issue.Title, nodeWidth-4, "…")

			node := graphNode{
				id:     id,
				title:  title,
				x:      x,
				y:      y,
				width:  nodeWidth,
				height: nodeHeight,
				layer:  layer,
				status: issue.Status,
				itype:  issue.IssueType,
			}

			nodeIdx := len(g.nodes)
			nodeIndexMap[id] = nodeIdx
			g.nodes = append(g.nodes, node)
			g.layers[layer][i] = nodeIdx
		}
	}

	// Create edges (from dependency TO dependent, so arrows point downward)
	for _, issue := range g.issues {
		dependentIdx, ok := nodeIndexMap[issue.ID]
		if !ok {
			continue
		}
		for _, dep := range issue.Dependencies {
			if dep.Type != model.DepBlocks && dep.Type != model.DepParentChild {
				continue
			}
			dependencyIdx, ok := nodeIndexMap[dep.DependsOnID]
			if !ok {
				continue
			}
			// Edge goes FROM the dependency (above) TO the dependent (below)
			g.edges = append(g.edges, graphEdge{
				from:    dependencyIdx,
				to:      dependentIdx,
				depType: string(dep.Type),
			})
		}
	}

	// Ensure selected node is valid
	if g.selectedNode >= len(g.nodes) {
		g.selectedNode = 0
	}
}

// Navigation methods
func (g *GraphModel) MoveUp() {
	if len(g.nodes) == 0 {
		return
	}
	// Move to previous node in same layer or previous layer
	current := g.nodes[g.selectedNode]
	currentLayer := current.layer

	// Try same layer, previous position
	for i := g.selectedNode - 1; i >= 0; i-- {
		if g.nodes[i].layer == currentLayer {
			g.selectedNode = i
			return
		}
	}

	// Try previous layer
	if currentLayer > 0 {
		for i := len(g.nodes) - 1; i >= 0; i-- {
			if g.nodes[i].layer == currentLayer-1 {
				g.selectedNode = i
				return
			}
		}
	}
}

func (g *GraphModel) MoveDown() {
	if len(g.nodes) == 0 {
		return
	}
	current := g.nodes[g.selectedNode]
	currentLayer := current.layer

	// Try same layer, next position
	for i := g.selectedNode + 1; i < len(g.nodes); i++ {
		if g.nodes[i].layer == currentLayer {
			g.selectedNode = i
			return
		}
	}

	// Try next layer
	for i := 0; i < len(g.nodes); i++ {
		if g.nodes[i].layer == currentLayer+1 {
			g.selectedNode = i
			return
		}
	}
}

func (g *GraphModel) MoveLeft() {
	if g.selectedNode > 0 {
		g.selectedNode--
	}
}

func (g *GraphModel) MoveRight() {
	if g.selectedNode < len(g.nodes)-1 {
		g.selectedNode++
	}
}

func (g *GraphModel) PageUp() {
	g.scrollY -= 10
	if g.scrollY < 0 {
		g.scrollY = 0
	}
}

func (g *GraphModel) PageDown() {
	g.scrollY += 10
	// Bound check will be applied during render based on canvas size
}

func (g *GraphModel) ScrollLeft() {
	g.scrollX -= 10
	if g.scrollX < 0 {
		g.scrollX = 0
	}
}

func (g *GraphModel) ScrollRight() {
	g.scrollX += 10
	// Bound check will be applied during render based on canvas size
}

// clampScroll ensures scroll values are within valid bounds
func (g *GraphModel) clampScroll(canvasWidth, canvasHeight, viewWidth, viewHeight int) {
	maxScrollX := canvasWidth - viewWidth
	if maxScrollX < 0 {
		maxScrollX = 0
	}
	if g.scrollX > maxScrollX {
		g.scrollX = maxScrollX
	}
	if g.scrollX < 0 {
		g.scrollX = 0
	}

	maxScrollY := canvasHeight - viewHeight
	if maxScrollY < 0 {
		maxScrollY = 0
	}
	if g.scrollY > maxScrollY {
		g.scrollY = maxScrollY
	}
	if g.scrollY < 0 {
		g.scrollY = 0
	}
}

// SelectedIssue returns the currently selected issue
func (g *GraphModel) SelectedIssue() *model.Issue {
	if len(g.nodes) == 0 || g.selectedNode >= len(g.nodes) {
		return nil
	}
	id := g.nodes[g.selectedNode].id
	return g.issueMap[id]
}

// TotalCount returns the number of nodes in the graph
func (g *GraphModel) TotalCount() int {
	return len(g.nodes)
}

// View renders the graph
func (g *GraphModel) View(width, height int) string {
	t := g.theme

	if len(g.nodes) == 0 {
		return t.Renderer.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Foreground(t.Secondary).
			Render("No dependency graph to display")
	}

	// Calculate canvas size based on nodes
	canvasWidth := 0
	canvasHeight := 0
	for _, node := range g.nodes {
		right := node.x + node.width + 2
		bottom := node.y + node.height + 2
		if right > canvasWidth {
			canvasWidth = right
		}
		if bottom > canvasHeight {
			canvasHeight = bottom
		}
	}

	// Minimum canvas size
	if canvasWidth < width {
		canvasWidth = width
	}
	if canvasHeight < height-4 {
		canvasHeight = height - 4
	}

	g.canvas = newCanvas(canvasWidth, canvasHeight)

	// Clamp scroll values before rendering
	viewHeight := height - 4
	g.clampScroll(canvasWidth, canvasHeight, width, viewHeight)

	// Draw edges first (behind nodes)
	g.drawEdges()

	// Draw nodes
	g.drawNodes()

	// Render header
	headerStyle := t.Renderer.NewStyle().
		Bold(true).
		Foreground(t.Primary)

	header := headerStyle.Render(fmt.Sprintf("📊 Dependency Graph (%d nodes, %d edges)", len(g.nodes), len(g.edges)))

	// Navigation hint
	navStyle := t.Renderer.NewStyle().
		Foreground(t.Secondary).
		Italic(true)
	nav := navStyle.Render("←↓↑→ navigate • PgUp/PgDn scroll • Enter select")

	// Scroll indicator
	scrollInfo := ""
	if g.scrollX > 0 || g.scrollY > 0 {
		scrollInfo = t.Renderer.NewStyle().
			Foreground(t.Feature). // Orange for scroll indicator
			Render(fmt.Sprintf(" [scroll: %d,%d]", g.scrollX, g.scrollY))
	}

	// Render canvas (viewHeight already computed for clampScroll)
	canvasView := g.canvas.render(t, g.scrollX, g.scrollY, width, viewHeight)

	return lipgloss.JoinVertical(lipgloss.Left,
		header+scrollInfo,
		nav,
		"",
		canvasView,
	)
}

// drawNodes renders all nodes onto the canvas
func (g *GraphModel) drawNodes() {
	t := g.theme

	for i, node := range g.nodes {
		isSelected := i == g.selectedNode
		g.drawNode(node, isSelected, t)
	}
}

// drawNode renders a single node
func (g *GraphModel) drawNode(node graphNode, selected bool, t Theme) {
	x, y := node.x, node.y
	w, h := node.width, node.height

	// Determine colors based on status
	var borderColor, textColor lipgloss.AdaptiveColor
	switch node.status {
	case model.StatusOpen:
		borderColor = t.Open
	case model.StatusInProgress:
		borderColor = t.InProgress
	case model.StatusBlocked:
		borderColor = t.Blocked
	case model.StatusClosed:
		borderColor = t.Closed
	default:
		borderColor = t.Secondary
	}
	textColor = borderColor

	borderStyle := t.Renderer.NewStyle().Foreground(borderColor)
	textStyle := t.Renderer.NewStyle().Foreground(textColor)
	idStyle := t.Renderer.NewStyle().Foreground(t.Secondary).Bold(true)

	if selected {
		borderStyle = t.Renderer.NewStyle().Foreground(t.Primary).Bold(true)
		textStyle = t.Renderer.NewStyle().Foreground(t.Primary).Bold(true)
	}

	// Draw box using Unicode
	// Top border: ╭─────╮
	g.canvas.set(x, y, '╭', borderStyle)
	for i := 1; i < w-1; i++ {
		g.canvas.set(x+i, y, '─', borderStyle)
	}
	g.canvas.set(x+w-1, y, '╮', borderStyle)

	// Middle rows
	for row := 1; row < h-1; row++ {
		g.canvas.set(x, y+row, '│', borderStyle)
		g.canvas.set(x+w-1, y+row, '│', borderStyle)
	}

	// Bottom border: ╰─────╯
	g.canvas.set(x, y+h-1, '╰', borderStyle)
	for i := 1; i < w-1; i++ {
		g.canvas.set(x+i, y+h-1, '─', borderStyle)
	}
	g.canvas.set(x+w-1, y+h-1, '╯', borderStyle)

	// Content: ID on first line, title on second
	icon, iconColor := t.GetTypeIcon(string(node.itype))
	iconStyle := t.Renderer.NewStyle().Foreground(iconColor)

	// Line 1: Icon + ID
	g.canvas.drawString(x+1, y+1, icon, iconStyle)
	g.canvas.drawString(x+3, y+1, node.id, idStyle)

	// Line 2: Title (if height allows)
	if h >= 3 {
		titleTrunc := truncateRunesHelper(node.title, w-2, "…")
		g.canvas.drawString(x+1, y+2, titleTrunc, textStyle)
	}
}

// drawEdges renders all edges onto the canvas
func (g *GraphModel) drawEdges() {
	t := g.theme
	edgeStyle := t.Renderer.NewStyle().Foreground(t.Secondary)

	for _, edge := range g.edges {
		if edge.from >= len(g.nodes) || edge.to >= len(g.nodes) {
			continue
		}
		fromNode := g.nodes[edge.from]
		toNode := g.nodes[edge.to]

		// Calculate connection points
		// From node: bottom center
		fromX := fromNode.x + fromNode.width/2
		fromY := fromNode.y + fromNode.height

		// To node: top center
		toX := toNode.x + toNode.width/2
		toY := toNode.y - 1

		// Draw the edge
		g.drawEdge(fromX, fromY, toX, toY, edgeStyle)
	}
}

// drawEdge draws a line between two points
func (g *GraphModel) drawEdge(x1, y1, x2, y2 int, style lipgloss.Style) {
	// Simple orthogonal routing: go down, then horizontal, then down again
	midY := (y1 + y2) / 2

	// Vertical line from start to mid
	for y := y1; y <= midY; y++ {
		existing := g.canvas.get(x1, y)
		char := '│'
		if existing == '─' {
			char = '┼'
		} else if existing == '╰' || existing == '╯' {
			char = '┴'
		} else if existing == '╭' || existing == '╮' {
			char = '┬'
		}
		g.canvas.set(x1, y, char, style)
	}

	// Horizontal line from x1 to x2 at midY
	if x1 != x2 {
		minX, maxX := x1, x2
		if x1 > x2 {
			minX, maxX = x2, x1
		}
		for x := minX; x <= maxX; x++ {
			existing := g.canvas.get(x, midY)
			char := '─'
			if existing == '│' {
				char = '┼'
			}
			g.canvas.set(x, midY, char, style)
		}
		// Corner at x1, midY
		if x1 < x2 {
			g.canvas.set(x1, midY, '└', style)
			g.canvas.set(x2, midY, '┐', style)
		} else {
			g.canvas.set(x1, midY, '┘', style)
			g.canvas.set(x2, midY, '┌', style)
		}
	}

	// Vertical line from mid to end
	// Start at midY+1 if there was a horizontal segment to avoid overwriting corner
	startY := midY
	if x1 != x2 {
		startY = midY + 1
	}
	for y := startY; y < y2; y++ {
		existing := g.canvas.get(x2, y)
		char := '│'
		if existing == '─' {
			char = '┼'
		}
		g.canvas.set(x2, y, char, style)
	}

	// Arrow at the end
	if y2 > 0 {
		g.canvas.set(x2, y2, '▼', style)
	}
}
