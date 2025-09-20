package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rescp17/lanFileSharer/internal/app_events/receiver"
	"github.com/rescp17/lanFileSharer/internal/style"
)

// SenderCardModel displays sender information and implements tea.Model
type SenderCardModel struct {
	OS       string
	Host     string // IP address
	Hostname string // hostname
}

// NewSenderCard returns a pointer to SenderCardModel so it can be used with tea.NewProgram
func NewSenderCard() *SenderCardModel {
	return &SenderCardModel{}
}

// Init implements tea.Model. No startup command needed here.
func (m SenderCardModel) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages. It supports SenderInfoMsg to update fields.
func (m SenderCardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case receiver.SenderUpdateMsg:
		m.OS = v.OS
		m.Host = v.Host
		m.Hostname = v.Hostname
	}
	return m, nil
}

// View renders the model as a string. Uses strings.Builder.WriteString / WriteByte for performance.
func (s SenderCardModel) View() string {
	var b strings.Builder

	osVal := style.RenderWithSafeReset(style.HighlightFontStyle, emptyIfZero(s.OS))
	hostVal := style.RenderWithSafeReset(style.HighlightFontStyle, emptyIfZero(s.Host))
	hostnameVal := style.RenderWithSafeReset(style.HighlightFontStyle, emptyIfZero(s.Hostname))

	b.WriteString("🏷️  Hostname:  ")
	b.WriteString(hostnameVal)
	b.WriteByte('\n')

	b.WriteString("🌐  Host (IP): ")
	b.WriteString(hostVal)
	b.WriteByte('\n')

	b.WriteString("💻  OS:        ")
	b.WriteString(osVal)
	b.WriteByte('\n')

	return b.String()
}

// emptyIfZero returns a placeholder when the input string is empty
func emptyIfZero(s string) string {
	if s == "" {
		return "<unknown>"
	}
	return s
}
