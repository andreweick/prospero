package topten

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// numberedItemRegex matches items that start with 1-2 digits followed by a period
var numberedItemRegex = regexp.MustCompile(`^\d{1,2}\.`)

// isAlreadyNumbered checks if the list items already contain numbers
func isAlreadyNumbered(items []string) bool {
	if len(items) == 0 {
		return false
	}
	return numberedItemRegex.MatchString(strings.TrimSpace(items[0]))
}

// PrintList prints a formatted Top 10 list to the provided writer
func PrintList(w io.Writer, list *TopTenList) {
	// Style definitions
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Align(lipgloss.Center).
		Padding(0, 2).
		Margin(1, 0).
		Border(lipgloss.RoundedBorder())

	dateStyle := lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#626262")).
		Align(lipgloss.Center).
		Margin(0, 0, 1, 0)

	numberStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF6B6B")).
		Bold(true)

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2).
		Margin(1, 0)

	// Create the title box
	titleBox := titleStyle.Render(list.Title)
	width := lipgloss.Width(titleBox)

	// Style the date to match the width of the title box
	styledDate := dateStyle.Width(width).Render(list.Date)

	// Build the content as a string first
	var content strings.Builder
	content.WriteString(titleBox)
	content.WriteString("\n")
	content.WriteString(styledDate)
	content.WriteString("\n\n")

	// Add the list items
	alreadyNumbered := isAlreadyNumbered(list.Items)
	for i, item := range list.Items {
		var number string
		var itemContent string

		if alreadyNumbered {
			// Items already have numbers, extract them
			parts := strings.SplitN(strings.TrimSpace(item), ".", 2)
			if len(parts) >= 2 {
				number = strings.TrimSpace(parts[0])
				itemContent = strings.TrimSpace(parts[1])
			} else {
				// Fallback if splitting fails
				number = fmt.Sprintf("%d", i+1)
				itemContent = strings.TrimSpace(item)
			}
		} else {
			// Add numbers counting down from 10 to 1
			number = fmt.Sprintf("%d", 10-i)
			itemContent = strings.TrimSpace(item)
		}

		// Right-align number within 2 character width
		formattedNumber := fmt.Sprintf("%2s.", number)
		styledNumber := numberStyle.Render(formattedNumber)
		content.WriteString(fmt.Sprintf("  %s %s\n", styledNumber, itemContent))
	}

	// Apply the container border and print
	finalOutput := containerStyle.Render(content.String())
	fmt.Fprintf(w, "\n%s\n\n", finalOutput)
}

// FormatListAsASCII returns a formatted Top 10 list as an ASCII string
func FormatListAsASCII(list *TopTenList) string {
	// Set ASCII mode for formatting
	lipgloss.SetColorProfile(termenv.Ascii)

	var buf bytes.Buffer
	PrintListASCII(&buf, list)
	return buf.String()
}

// PrintListASCII prints a formatted Top 10 list to the provided writer in ASCII mode
func PrintListASCII(w io.Writer, list *TopTenList) {
	// Style definitions for ASCII mode
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Align(lipgloss.Center).
		Padding(0, 2).
		Margin(1, 0).
		Border(lipgloss.ASCIIBorder())

	dateStyle := lipgloss.NewStyle().
		Italic(true).
		Align(lipgloss.Center).
		Margin(0, 0, 1, 0)

	numberStyle := lipgloss.NewStyle().
		Bold(true)

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.ASCIIBorder()).
		Padding(1, 2).
		Margin(1, 0)

	// Create the title box
	titleBox := titleStyle.Render(list.Title)
	width := lipgloss.Width(titleBox)

	// Style the date to match the width of the title box
	styledDate := dateStyle.Width(width).Render(list.Date)

	// Build the content as a string first
	var content strings.Builder
	content.WriteString(titleBox)
	content.WriteString("\n")
	content.WriteString(styledDate)
	content.WriteString("\n\n")

	// Add the list items
	alreadyNumbered := isAlreadyNumbered(list.Items)
	for i, item := range list.Items {
		var number string
		var itemContent string

		if alreadyNumbered {
			// Items already have numbers, extract them
			parts := strings.SplitN(strings.TrimSpace(item), ".", 2)
			if len(parts) >= 2 {
				number = strings.TrimSpace(parts[0])
				itemContent = strings.TrimSpace(parts[1])
			} else {
				// Fallback if splitting fails
				number = fmt.Sprintf("%d", i+1)
				itemContent = strings.TrimSpace(item)
			}
		} else {
			// Add numbers counting down from 10 to 1
			number = fmt.Sprintf("%d", 10-i)
			itemContent = strings.TrimSpace(item)
		}

		// Right-align number within 2 character width
		formattedNumber := fmt.Sprintf("%2s.", number)
		styledNumber := numberStyle.Render(formattedNumber)
		content.WriteString(fmt.Sprintf("  %s %s\n", styledNumber, itemContent))
	}

	// Apply the container border and print
	finalOutput := containerStyle.Render(content.String())
	fmt.Fprintf(w, "\n%s\n\n", finalOutput)
}
