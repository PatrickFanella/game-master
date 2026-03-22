// Package styles defines the shared visual theme for the game-master TUI using
// Lip Gloss. It provides a color palette, border styles, text styles (headers,
// body, player input, NPC dialogue, system messages), and layout helpers for
// consistent padding, margins, and maximum widths across all views.
package styles

import "github.com/charmbracelet/lipgloss"

// ---------------------------------------------------------------------------
// Color palette
// ---------------------------------------------------------------------------
// Each color is an AdaptiveColor so the TUI looks correct in both light and
// dark terminals: the first value is used for dark terminals, the second for
// light terminals.

var (
	// Background / surface tones.
	ColorBackground = lipgloss.AdaptiveColor{Dark: "#1a1a2e", Light: "#f8f9fa"}
	ColorSurface    = lipgloss.AdaptiveColor{Dark: "#16213e", Light: "#e9ecef"}
	ColorBorder     = lipgloss.AdaptiveColor{Dark: "#4a4a6a", Light: "#ced4da"}

	// Primary foreground / body text.
	ColorForeground = lipgloss.AdaptiveColor{Dark: "#e0e0f0", Light: "#212529"}
	ColorMuted      = lipgloss.AdaptiveColor{Dark: "#6b7280", Light: "#868e96"}

	// Accent / highlight.
	ColorAccent    = lipgloss.AdaptiveColor{Dark: "#c084fc", Light: "#7c3aed"}
	ColorAccentDim = lipgloss.AdaptiveColor{Dark: "#7c3aed", Light: "#a78bfa"}

	// Semantic colors.
	ColorSuccess = lipgloss.AdaptiveColor{Dark: "#4ade80", Light: "#16a34a"}
	ColorWarning = lipgloss.AdaptiveColor{Dark: "#facc15", Light: "#d97706"}
	ColorError   = lipgloss.AdaptiveColor{Dark: "#f87171", Light: "#dc2626"}
	ColorInfo    = lipgloss.AdaptiveColor{Dark: "#60a5fa", Light: "#2563eb"}

	// Role-specific text colors.
	ColorPlayerInput  = lipgloss.AdaptiveColor{Dark: "#34d399", Light: "#15803d"}
	ColorNPCDialogue  = lipgloss.AdaptiveColor{Dark: "#fcd34d", Light: "#b45309"}
	ColorSystemMsg    = lipgloss.AdaptiveColor{Dark: "#94a3b8", Light: "#6b7280"}
	ColorHeaderText   = lipgloss.AdaptiveColor{Dark: "#c084fc", Light: "#7c3aed"}
)

// ---------------------------------------------------------------------------
// Border styles
// ---------------------------------------------------------------------------

// BorderNormal is the default rounded border for view containers.
var BorderNormal = lipgloss.Border{
	Top:         "─",
	Bottom:      "─",
	Left:        "│",
	Right:       "│",
	TopLeft:     "╭",
	TopRight:    "╮",
	BottomLeft:  "╰",
	BottomRight: "╯",
}

// BorderDouble is used for emphasized / focused containers.
var BorderDouble = lipgloss.Border{
	Top:         "═",
	Bottom:      "═",
	Left:        "║",
	Right:       "║",
	TopLeft:     "╔",
	TopRight:    "╗",
	BottomLeft:  "╚",
	BottomRight: "╝",
}

// BorderThick is used for the outermost application frame.
var BorderThick = lipgloss.ThickBorder()

// ---------------------------------------------------------------------------
// Base / reset style
// ---------------------------------------------------------------------------

// Base is the zero-value starting point for all other styles.
var Base = lipgloss.NewStyle()

// ---------------------------------------------------------------------------
// Text styles
// ---------------------------------------------------------------------------

// Header renders a section heading (bold, accent color).
var Header = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorHeaderText)

// SubHeader renders a secondary heading (bold, muted accent).
var SubHeader = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorAccentDim)

// Body is the standard paragraph / body-copy style.
var Body = lipgloss.NewStyle().
	Foreground(ColorForeground)

// Muted renders de-emphasized helper text.
var Muted = lipgloss.NewStyle().
	Foreground(ColorMuted)

// PlayerInput styles the player's own typed text with a distinctive prefix.
var PlayerInput = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorPlayerInput)

// PlayerInputPrefix is the ">" symbol prepended to player input lines.
var PlayerInputPrefix = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorPlayerInput).
	SetString("> ")

// NPCDialogue styles spoken NPC dialogue in warm amber.
var NPCDialogue = lipgloss.NewStyle().
	Italic(true).
	Foreground(ColorNPCDialogue)

// NPCName renders an NPC's name label in bold amber.
var NPCName = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorNPCDialogue)

// SystemMessage styles out-of-character / system notifications.
var SystemMessage = lipgloss.NewStyle().
	Italic(true).
	Foreground(ColorSystemMsg)

// Success / Warning / Error / Info inline badges.

// StatusSuccess renders a positive status label.
var StatusSuccess = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorSuccess)

// StatusWarning renders a cautionary status label.
var StatusWarning = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorWarning)

// StatusError renders an error or danger label.
var StatusError = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorError)

// StatusInfo renders an informational label.
var StatusInfo = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorInfo)

// ---------------------------------------------------------------------------
// Container / box styles
// ---------------------------------------------------------------------------

// Container wraps a view in a rounded border with default padding.
var Container = lipgloss.NewStyle().
	Border(BorderNormal).
	BorderForeground(ColorBorder).
	Padding(0, 1)

// FocusedContainer is like Container but uses the accent color for its border.
var FocusedContainer = lipgloss.NewStyle().
	Border(BorderNormal).
	BorderForeground(ColorAccent).
	Padding(0, 1)

// Panel is a borderless box with consistent inner padding.
var Panel = lipgloss.NewStyle().
	Padding(1, 2)

// TitleBar is the application title banner at the very top.
var TitleBar = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorHeaderText).
	Background(ColorSurface).
	Padding(0, 2).
	Border(lipgloss.NormalBorder(), false, false, true, false).
	BorderForeground(ColorBorder)

// StatusBar is the thin bar at the bottom of the screen showing key hints.
var StatusBar = lipgloss.NewStyle().
	Foreground(ColorMuted).
	Background(ColorSurface).
	Padding(0, 1).
	Border(lipgloss.NormalBorder(), true, false, false, false).
	BorderForeground(ColorBorder)

// Tab renders an inactive tab label.
var Tab = lipgloss.NewStyle().
	Foreground(ColorMuted).
	Padding(0, 2)

// ActiveTab renders the currently selected tab label.
var ActiveTab = lipgloss.NewStyle().
	Bold(true).
	Foreground(ColorAccent).
	Padding(0, 2).
	Border(lipgloss.NormalBorder(), false, false, true, false).
	BorderForeground(ColorAccent)

// ---------------------------------------------------------------------------
// Layout helpers
// ---------------------------------------------------------------------------

// MaxWidth clamps the rendered width of a style.
func MaxWidth(s lipgloss.Style, w int) lipgloss.Style {
	return s.MaxWidth(w)
}

// WithPadding adds uniform horizontal and vertical padding to a style.
func WithPadding(s lipgloss.Style, vertical, horizontal int) lipgloss.Style {
	return s.Padding(vertical, horizontal)
}

// WithMargin adds uniform horizontal and vertical margin to a style.
func WithMargin(s lipgloss.Style, vertical, horizontal int) lipgloss.Style {
	return s.Margin(vertical, horizontal)
}

// Place centres content inside a box of the given dimensions.
func Place(width, height int, content string) string {
	return lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center,
		content)
}

// JoinHorizontal joins rendered blocks side-by-side at the top edge.
func JoinHorizontal(blocks ...string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, blocks...)
}

// JoinVertical stacks rendered blocks from top to bottom, left-aligned.
func JoinVertical(blocks ...string) string {
	return lipgloss.JoinVertical(lipgloss.Left, blocks...)
}
