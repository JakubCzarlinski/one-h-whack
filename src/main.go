package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Simple translation map for common terms
var translations = map[string]string{
	"Desktop":   "Bureau",
	"Documents": "Documents",
	"Downloads": "Téléchargements",
	"Pictures":  "Images",
	"Music":     "Musique",
	"Videos":    "Vidéos",
	"Home":      "Accueil",
	"Folder":    "Dossier",
	"File":      "Fichier",
}

type item struct {
	title       string
	translation string
	path        string
	isDir       bool
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.translation }
func (i item) FilterValue() string { return i.title }

type model struct {
	list        list.Model
	currentPath string
	err         error
	quitting    bool
}

func translateName(name string) string {
	// Check direct translation
	if trans, ok := translations[name]; ok {
		return trans
	}

	// Try case-insensitive
	for key, val := range translations {
		if strings.EqualFold(key, name) {
			return val
		}
	}

	// Basic word replacements
	result := name
	result = strings.ReplaceAll(result, "file", "fichier")
	result = strings.ReplaceAll(result, "folder", "dossier")
	result = strings.ReplaceAll(result, "new", "nouveau")
	result = strings.ReplaceAll(result, "old", "ancien")

	return result
}

func getDirectoryItems(path string) ([]list.Item, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	items := make([]list.Item, 0)

	// Add parent directory option if not at root
	if path != "/" && path != filepath.Dir(path) {
		items = append(items, item{
			title:       "..",
			translation: "Dossier parent",
			path:        filepath.Dir(path),
			isDir:       true,
		})
	}

	for _, entry := range entries {
		// Skip hidden files on Unix-like systems
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		translation := translateName(entry.Name())
		typeStr := "Fichier"
		if entry.IsDir() {
			typeStr = "Dossier"
		}

		items = append(items, item{
			title:       entry.Name(),
			translation: fmt.Sprintf("%s → %s", typeStr, translation),
			path:        filepath.Join(path, entry.Name()),
			isDir:       entry.IsDir(),
		})
	}

	return items, nil
}

func initialModel() model {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	items, err := getDirectoryItems(homeDir)
	if err != nil {
		items = []list.Item{}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("170")).
		Bold(true)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("141"))

	l := list.New(items, delegate, 0, 0)
	l.Title = fmt.Sprintf("Navigateur de fichiers - %s", homeDir)
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1)

	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "ouvrir"),
			),
		}
	}

	return model{
		list:        l,
		currentPath: homeDir,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			selectedItem, ok := m.list.SelectedItem().(item)
			if !ok {
				return m, nil
			}

			if selectedItem.isDir {
				// Navigate into directory
				items, err := getDirectoryItems(selectedItem.path)
				if err != nil {
					m.err = err
					return m, nil
				}

				m.currentPath = selectedItem.path
				m.list.Title = fmt.Sprintf("Navigateur de fichiers - %s",
					m.currentPath)
				m.list.SetItems(items)
				m.list.ResetSelected()
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		h, v := lipgloss.NewStyle().GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return "Au revoir! 👋\n"
	}

	if m.err != nil {
		return fmt.Sprintf("Erreur: %v\n\nAppuyez sur 'q' pour quitter.\n",
			m.err)
	}

	return m.list.View()
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Erreur: %v\n", err)
		os.Exit(1)
	}
}
