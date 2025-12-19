package packagist

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Requirements представляет гибкий тип для require/require-dev
type Requirements map[string]string

// UnmarshalJSON позволяет парсить как map, так и игнорировать невалидные значения
func (r *Requirements) UnmarshalJSON(data []byte) error {
	// Пытаемся распарсить как map
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		// Если не получилось - возвращаем пустой map
		*r = make(Requirements)
		return nil
	}
	*r = m
	return nil
}

// AutoloadConfig представляет гибкую конфигурацию автозагрузки
type AutoloadConfig map[string]interface{}

// UnmarshalJSON позволяет парсить autoload в разных форматах
func (a *AutoloadConfig) UnmarshalJSON(data []byte) error {
	// Пытаемся распарсить как map
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		// Если не получилось (строка, null и т.д.) - возвращаем пустой map
		*a = make(AutoloadConfig)
		return nil
	}
	*a = m
	return nil
}

// FundingInfo представляет гибкую информацию о финансировании
type FundingInfo []map[string]string

// UnmarshalJSON позволяет парсить funding в разных форматах
func (f *FundingInfo) UnmarshalJSON(data []byte) error {
	// Пытаемся распарсить как массив
	var arr []map[string]string
	if err := json.Unmarshal(data, &arr); err != nil {
		// Если не получилось (строка, null и т.д.) - возвращаем пустой массив
		*f = []map[string]string{}
		return nil
	}
	*f = arr
	return nil
}

// FlexibleMap представляет гибкий тип для map[string]string или string/array
type FlexibleMap map[string]string

// UnmarshalJSON позволяет парсить как map, string или array
func (f *FlexibleMap) UnmarshalJSON(data []byte) error {
	// Пытаемся распарсить как map
	var m map[string]string
	if err := json.Unmarshal(data, &m); err == nil {
		*f = m
		return nil
	}
	
	// Пытаемся как строку - игнорируем
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = make(FlexibleMap)
		return nil
	}
	
	// Пытаемся как array - игнорируем
	var arr []interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		*f = make(FlexibleMap)
		return nil
	}
	
	// Если ничего не подошло - возвращаем пустой map
	*f = make(FlexibleMap)
	return nil
}

const (
	DefaultPackagistURL = "https://repo.packagist.org"
)

// Client представляет клиент для Packagist API
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient создает новый клиент Packagist
func NewClient() *Client {
	return &Client{
		BaseURL: DefaultPackagistURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// PackageInfo содержит информацию о пакете из Packagist
type PackageInfo struct {
	Packages map[string][]PackageVersion `json:"packages"`
}

// PackageVersion представляет конкретную версию пакета
type PackageVersion struct {
	Name              string            `json:"name"`
	Version           string            `json:"version"`
	VersionNormalized string            `json:"version_normalized,omitempty"`
	Description       string            `json:"description,omitempty"`
	Keywords          []string          `json:"keywords,omitempty"`
	Homepage          string            `json:"homepage,omitempty"`
	Type              string            `json:"type,omitempty"`
	License           []string          `json:"license,omitempty"`
	Authors           []Author          `json:"authors,omitempty"`
	Source            *Source           `json:"source,omitempty"`
	Dist              FlexibleDist      `json:"dist,omitempty"`
	Require           Requirements      `json:"require,omitempty"`
	RequireDev        Requirements      `json:"require-dev,omitempty"`
	Replace           FlexibleMap       `json:"replace,omitempty"`
	Autoload          AutoloadConfig    `json:"autoload,omitempty"`
	Time              string            `json:"time,omitempty"`
	Support           map[string]string `json:"support,omitempty"`
	Funding           FundingInfo       `json:"funding,omitempty"`
}

// Author представляет автора пакета
type Author struct {
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Homepage string `json:"homepage,omitempty"`
	Role     string `json:"role,omitempty"`
}

// Source представляет источник пакета
type Source struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Reference string `json:"reference"`
}

// Dist представляет дистрибутив пакета (гибкий тип)
type Dist struct {
	Type      string `json:"type"`
	URL       string `json:"url"`
	Reference string `json:"reference,omitempty"`
	Shasum    string `json:"shasum,omitempty"`
}

// FlexibleDist - обертка для Dist, которая может быть объектом или строкой
type FlexibleDist struct {
	*Dist
}

// UnmarshalJSON позволяет парсить dist как объект, строку или null
func (f *FlexibleDist) UnmarshalJSON(data []byte) error {
	// Пытаемся распарсить как объект
	var d Dist
	if err := json.Unmarshal(data, &d); err == nil {
		f.Dist = &d
		return nil
	}
	
	// Пытаемся как строку - игнорируем
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		f.Dist = nil
		return nil
	}
	
	// Если null или другой тип - тоже игнорируем
	f.Dist = nil
	return nil
}

// GetPackage получает информацию о пакете
func (c *Client) GetPackage(name string) (*PackageInfo, error) {
	url := fmt.Sprintf("%s/p2/%s.json", c.BaseURL, name)

	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("packagist returned status %d for package %s", resp.StatusCode, name)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var info PackageInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse package info: %w", err)
	}

	return &info, nil
}

// DownloadPackage загружает дистрибутив пакета
func (c *Client) DownloadPackage(url string) ([]byte, error) {
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download package: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read package data: %w", err)
	}

	return data, nil
}
