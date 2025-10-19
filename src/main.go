package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	gt "gopkg.gilang.dev/google-translate"
	"gopkg.gilang.dev/google-translate/params"
)

// Translation cache to avoid repeated API calls
type translationCache struct {
	mu    sync.RWMutex
	cache map[string]string
}

func (tc *translationCache) Get(key string) (string, bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	val, ok := tc.cache[key]
	return val, ok
}

func (tc *translationCache) Set(key, val string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cache[key] = val
}

var (
	// Pre-defined translations for common terms
	staticTranslations = map[string]string{
		"Desktop":      "Bureau",
		"Documents":    "Documents",
		"Downloads":    "Téléchargements",
		"Pictures":     "Images",
		"Music":        "Musique",
		"Videos":       "Vidéos",
		"Home":         "Accueil",
		"Folder":       "Dossier",
		"File":         "Fichier",
		"Public":       "Public",
		"Templates":    "Modèles",
		"Library":      "Bibliothèque",
		"Applications": "Applications",
		"Movies":       "Films",
		"bin":          "binaire",
		"src":          "source",
		"pkg":          "paquets",
		"tmp":          "temporaire",
		"opt":          "optionnel",
		"usr":          "utilisateur",
		"var":          "variable",
		"etc":          "configuration",
	}

	// Global translation cache and client
	cache = &translationCache{
		cache: make(map[string]string),
	}
)

type translationMsg struct {
	name        string
	translation string
}

type renameCompleteMsg struct {
	oldPath string
	newPath string
	success bool
	err     error
}

type item struct {
	title       string
	translation string
	path        string
	isDir       bool
	translating bool
}

func (i item) Title() string {
	if i.translating {
		return i.title + " ⏳"
	}
	if i.isDir {
		return i.title + " 📁"
	}
	return i.title
}

func (i item) Description() string {
	if i.translation == "" {
		return "Traduction en cours..."
	}
	return i.translation
}

func (i item) FilterValue() string { return i.title }

type viewMode int

const (
	normalMode viewMode = iota
	confirmRenameMode
)

type model struct {
	list          list.Model
	currentPath   string
	err           error
	quitting      bool
	mode          viewMode
	confirmInput  textinput.Model
	itemToRename  item
	proposedName  string
	statusMessage string
}

func translateText(text string) (string, error) {
	// Check cache first
	if trans, ok := cache.Get(text); ok {
		return trans, nil
	}

	// Check static translations
	if trans, ok := staticTranslations[text]; ok {
		cache.Set(text, trans)
		return trans, nil
	}

	// Case-insensitive check
	for key, val := range staticTranslations {
		if strings.EqualFold(key, text) {
			cache.Set(text, val)
			return val, nil
		}
	}

	// Remove file extensions for better translation
	nameWithoutExt := strings.TrimSuffix(text, filepath.Ext(text))
	ext := filepath.Ext(text)

	// Skip translation for very short names or single characters
	if len(nameWithoutExt) <= 1 {
		cache.Set(text, text)
		return text, nil
	}

	// Skip translation for names that are all numbers or special chars
	if !containsLetters(nameWithoutExt) {
		cache.Set(text, text)
		return text, nil
	}

	// Use defer/recover to catch panics from the translation library
	var translated string
	var err error

	func() {
		defer func() {
			if r := recover(); r != nil {
				// If translation panics, just use the original text
				err = fmt.Errorf("translation panic: %v", r)
			}
		}()

		value := params.Translate{
			Text: nameWithoutExt,
			From: "en",
			To:   "fr",
		}

		result, translateErr := gt.TranslateWithParam(value)
		if translateErr != nil {
			err = translateErr
			return
		}
		translated = result.Text + ext
	}()

	if err != nil {
		// Fallback to original text on error
		cache.Set(text, text)
		return text, nil
	}

	cache.Set(text, translated)
	return translated, nil
}

func containsLetters(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

func translateNameCmd(name string) tea.Cmd {
	return func() tea.Msg {
		translation, err := translateText(name)
		if err != nil {
			translation = name // Fallback to original
		}
		return translationMsg{
			name:        name,
			translation: translation,
		}
	}
}

func renameFileCmd(oldPath, newPath string) tea.Cmd {
	return func() tea.Msg {
		err := os.Rename(oldPath, newPath)
		return renameCompleteMsg{
			oldPath: oldPath,
			newPath: newPath,
			success: err == nil,
			err:     err,
		}
	}
}

func getDirectoryItems(path string) ([]list.Item, []tea.Cmd) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, nil
	}

	items := make([]list.Item, 0)
	cmds := make([]tea.Cmd, 0)

	for _, entry := range entries {
		// Skip hidden files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		typeStr := "Fichier"
		if entry.IsDir() {
			typeStr = "Dossier"
		}

		// Check if translation is cached
		cached, hasCached := cache.Get(entry.Name())

		newItem := item{
			title:       entry.Name(),
			translation: "",
			path:        filepath.Join(path, entry.Name()),
			isDir:       entry.IsDir(),
			translating: !hasCached,
		}

		if hasCached {
			newItem.translation = fmt.Sprintf("%s → %s", typeStr, cached)
		} else {
			newItem.translation = fmt.Sprintf("%s → ...", typeStr)
			// Queue translation
			cmds = append(cmds, translateNameCmd(entry.Name()))
		}

		items = append(items, newItem)
	}

	return items, cmds
}

func initialModel() model {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	items, _ := getDirectoryItems(homeDir)

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
				key.WithKeys("left"),
				key.WithHelp("←", "parent"),
			),
			key.NewBinding(
				key.WithKeys("right"),
				key.WithHelp("→", "ouvrir"),
			),
			key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "renommer"),
			),
		}
	}

	ti := textinput.New()
	ti.Placeholder = "Nouveau nom..."
	ti.CharLimit = 255

	return model{
		list:         l,
		currentPath:  homeDir,
		mode:         normalMode,
		confirmInput: ti,
	}
}

func (m model) Init() tea.Cmd {
	// Trigger initial translations
	items, cmds := getDirectoryItems(m.currentPath)
	m.list.SetItems(items)
	return tea.Batch(cmds...)
}

func (m model) navigateToDirectory(path string) (model, tea.Cmd) {
	items, cmds := getDirectoryItems(path)
	m.currentPath = path
	m.list.Title = fmt.Sprintf("Navigateur de fichiers - %s", m.currentPath)
	m.list.SetItems(items)
	m.list.ResetSelected()
	m.statusMessage = ""
	return m, tea.Batch(cmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case translationMsg:
		// Update the item with the translation
		items := m.list.Items()
		for i, itm := range items {
			if listItem, ok := itm.(item); ok {
				if listItem.title == msg.name {
					typeStr := "Fichier"
					if listItem.isDir {
						typeStr = "Dossier"
					}
					listItem.translation = fmt.Sprintf("%s → %s",
						typeStr, msg.translation)
					listItem.translating = false
					items[i] = listItem
					break
				}
			}
		}
		m.list.SetItems(items)
		return m, nil

	case renameCompleteMsg:
		m.mode = normalMode
		if msg.success {
			m.statusMessage = fmt.Sprintf("✓ Renommé: %s → %s",
				filepath.Base(msg.oldPath), filepath.Base(msg.newPath))
			// Refresh directory
			return m.navigateToDirectory(m.currentPath)
		} else {
			m.statusMessage = fmt.Sprintf("✗ Erreur: %v", msg.err)
			return m, nil
		}

	case tea.KeyMsg:
		if m.mode == confirmRenameMode {
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				m.statusMessage = ""
				return m, nil

			case "enter":
				newName := m.confirmInput.Value()
				if newName == "" {
					newName = m.proposedName
				}

				oldPath := m.itemToRename.path
				newPath := filepath.Join(filepath.Dir(oldPath), newName)

				m.statusMessage = "Renommage en cours..."
				return m, renameFileCmd(oldPath, newPath)

			default:
				var cmd tea.Cmd
				m.confirmInput, cmd = m.confirmInput.Update(msg)
				return m, cmd
			}
		}

		// Normal mode
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "left":
			// Navigate to parent directory
			parentPath := filepath.Dir(m.currentPath)
			// Prevent going beyond root
			if parentPath != m.currentPath {
				return m.navigateToDirectory(parentPath)
			}
			return m, nil

		case "enter":
			// Show rename confirmation
			selectedItem, ok := m.list.SelectedItem().(item)
			if !ok {
				return m, nil
			}

			// Get the French translation
			translation, _ := cache.Get(selectedItem.title)
			if translation == "" || translation == selectedItem.title {
				m.statusMessage = "✗ Pas de traduction disponible"
				return m, nil
			}

			m.mode = confirmRenameMode
			m.itemToRename = selectedItem
			m.proposedName = translation
			m.confirmInput.SetValue(translation)
			m.confirmInput.Focus()
			m.statusMessage = ""

			return m, textinput.Blink

		case "right":
			// Navigate into directory
			selectedItem, ok := m.list.SelectedItem().(item)
			if !ok {
				return m, nil
			}

			if selectedItem.isDir {
				return m.navigateToDirectory(selectedItem.path)
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

	if m.mode == confirmRenameMode {
		confirmStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2).
			Width(60)

		content := fmt.Sprintf(
			"Renommer:\n\n"+
				"  Ancien: %s\n"+
				"  Nouveau: %s\n\n"+
				"Modifier le nom:\n%s\n\n"+
				"[Enter] Confirmer  [Esc] Annuler",
			m.itemToRename.title,
			m.proposedName,
			m.confirmInput.View(),
		)

		return lipgloss.Place(
			m.list.Width(),
			m.list.Height(),
			lipgloss.Center,
			lipgloss.Center,
			confirmStyle.Render(content),
		)
	}

	view := m.list.View()

	if m.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("170")).
			Bold(true).
			Padding(0, 1)
		view += "\n" + statusStyle.Render(m.statusMessage)
	}

	return view
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Erreur: %v\n", err)
		os.Exit(1)
	}
}
