package composer

import (
	"encoding/json"
	"os"
)

// StabilityFlags - гибкий тип для stability-flags (может быть map или array)
type StabilityFlags map[string]int

// UnmarshalJSON для StabilityFlags - обрабатывает как map, так и array
func (s *StabilityFlags) UnmarshalJSON(data []byte) error {
	// Попробуем распарсить как map
	var m map[string]int
	if err := json.Unmarshal(data, &m); err == nil {
		*s = m
		return nil
	}
	
	// Попробуем как array - игнорируем
	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		*s = make(StabilityFlags)
		return nil
	}
	
	// Если ничего не подошло, возвращаем пустой map
	*s = make(StabilityFlags)
	return nil
}

// PlatformPackages - гибкий тип для platform/platform-dev (может быть map или array)
type PlatformPackages map[string]string

// UnmarshalJSON для PlatformPackages - обрабатывает как map, так и array
func (p *PlatformPackages) UnmarshalJSON(data []byte) error {
	// Попробуем распарсить как map
	var m map[string]string
	if err := json.Unmarshal(data, &m); err == nil {
		*p = m
		return nil
	}
	
	// Попробуем как array - игнорируем
	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		*p = make(PlatformPackages)
		return nil
	}
	
	// Если ничего не подошло, возвращаем пустой map
	*p = make(PlatformPackages)
	return nil
}

// ComposerLock представляет структуру composer.lock
type ComposerLock struct {
	ReadmeFile       interface{}      `json:"_readme,omitempty"` // Может быть string или []string
	ContentHash      string           `json:"content-hash"`
	Packages         []LockedPackage  `json:"packages"`
	PackagesDev      []LockedPackage  `json:"packages-dev"`
	Aliases          []interface{}    `json:"aliases"`
	MinimumStability string           `json:"minimum-stability,omitempty"`
	StabilityFlags   StabilityFlags   `json:"stability-flags,omitempty"`
	PreferStable     bool             `json:"prefer-stable,omitempty"`
	PreferLowest     bool             `json:"prefer-lowest,omitempty"`
	Platform         PlatformPackages `json:"platform,omitempty"`
	PlatformDev      PlatformPackages `json:"platform-dev,omitempty"`
	PluginAPIVersion string           `json:"plugin-api-version,omitempty"`
}

// LockedPackage представляет заблокированный пакет
type LockedPackage struct {
	Name             string                 `json:"name"`
	Version          string                 `json:"version"`
	Source           *Source                `json:"source,omitempty"`
	Dist             *Dist                  `json:"dist,omitempty"`
	Require          map[string]string      `json:"require,omitempty"`
	RequireDev       map[string]string      `json:"require-dev,omitempty"`
	Type             string                 `json:"type,omitempty"`
	Autoload         AutoloadConfig         `json:"autoload,omitempty"`
	NotificationURL  string                 `json:"notification-url,omitempty"`
	License          []string               `json:"license,omitempty"`
	Authors          []Author               `json:"authors,omitempty"`
	Description      string                 `json:"description,omitempty"`
	Homepage         string                 `json:"homepage,omitempty"`
	Keywords         []string               `json:"keywords,omitempty"`
	Time             string                 `json:"time,omitempty"`
	Support          map[string]string      `json:"support,omitempty"`
	Funding          []map[string]string    `json:"funding,omitempty"`
}

// Source представляет источник пакета (git, svn и т.д.)
type Source struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Reference string `json:"reference"`
}

// Dist представляет дистрибутив пакета (zip, tar и т.д.)
type Dist struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Reference string `json:"reference,omitempty"`
	Shasum    string `json:"shasum,omitempty"`
}

// LoadComposerLock загружает и парсит composer.lock
func LoadComposerLock(path string) (*ComposerLock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lock ComposerLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	return &lock, nil
}

// Save сохраняет composer.lock
func (l *ComposerLock) Save(path string) error {
	// Устанавливаем readme
	if l.ReadmeFile == "" {
		l.ReadmeFile = []string{
			"This file locks the dependencies of your project to a known state",
			"Read more about it at https://getcomposer.org/doc/01-basic-usage.md#installing-dependencies",
			"This file is @generated automatically",
		}[0]
	}

	data, err := json.MarshalIndent(l, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// NewComposerLock создает новый composer.lock
func NewComposerLock(contentHash string) *ComposerLock {
	return &ComposerLock{
		ContentHash:  contentHash,
		Packages:     []LockedPackage{},
		PackagesDev:  []LockedPackage{},
		Aliases:      []interface{}{},
		ReadmeFile:   "This file locks the dependencies of your project to a known state",
	}
}

