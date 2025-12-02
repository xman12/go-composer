package composer

import (
	"encoding/json"
	"os"
)

// Scripts представляет секцию scripts в composer.json
type Scripts map[string]interface{}

// GetScripts возвращает список команд для указанного события
func (s Scripts) GetScripts(event string) []string {
	if s == nil {
		return nil
	}
	
	value, ok := s[event]
	if !ok {
		return nil
	}
	
	// Может быть строкой или массивом строк
	switch v := value.(type) {
	case string:
		return []string{v}
	case []interface{}:
		scripts := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				scripts = append(scripts, str)
			}
		}
		return scripts
	case []string:
		return v
	}
	
	return nil
}

// ComposerJSON представляет структуру composer.json
type ComposerJSON struct {
	Name        string                       `json:"name,omitempty"`
	Description string                       `json:"description,omitempty"`
	Type        string                       `json:"type,omitempty"`
	License     string                       `json:"license,omitempty"`
	Authors     []Author                     `json:"authors,omitempty"`
	Require     map[string]string            `json:"require,omitempty"`
	RequireDev  map[string]string            `json:"require-dev,omitempty"`
	Autoload    AutoloadConfig               `json:"autoload,omitempty"`
	AutoloadDev AutoloadConfig               `json:"autoload-dev,omitempty"`
	Repositories []Repository                `json:"repositories,omitempty"`
	Config      map[string]interface{}       `json:"config,omitempty"`
	Scripts     Scripts                      `json:"scripts,omitempty"`
	Extra       map[string]interface{}       `json:"extra,omitempty"`
}

// Author представляет автора пакета
type Author struct {
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Homepage string `json:"homepage,omitempty"`
	Role     string `json:"role,omitempty"`
}

// AutoloadConfig содержит конфигурацию автозагрузки
type AutoloadConfig struct {
	PSR4       map[string]interface{} `json:"psr-4,omitempty"`
	PSR0       map[string]interface{} `json:"psr-0,omitempty"`
	Classmap   []string               `json:"classmap,omitempty"`
	Files      []string               `json:"files,omitempty"`
	ExcludeFromClassmap []string      `json:"exclude-from-classmap,omitempty"`
}

// Repository представляет репозиторий пакетов
type Repository struct {
	Type string `json:"type"`
	URL  string `json:"url,omitempty"`
}

// LoadComposerJSON загружает и парсит composer.json
func LoadComposerJSON(path string) (*ComposerJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var composer ComposerJSON
	if err := json.Unmarshal(data, &composer); err != nil {
		return nil, err
	}

	return &composer, nil
}

// Save сохраняет composer.json
func (c *ComposerJSON) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

