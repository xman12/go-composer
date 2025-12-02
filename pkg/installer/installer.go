package installer

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aleksandrbelysev/go-composer/pkg/composer"
	"github.com/aleksandrbelysev/go-composer/pkg/packagist"
	"github.com/aleksandrbelysev/go-composer/pkg/resolver"
	"github.com/schollz/progressbar/v3"
)

// Installer управляет установкой пакетов
type Installer struct {
	client    *packagist.Client
	resolver  *resolver.Resolver
	vendorDir string
}

// NewInstaller создает новый installer
func NewInstaller(vendorDir string) *Installer {
	client := packagist.NewClient()
	return &Installer{
		client:    client,
		resolver:  resolver.NewResolver(client),
		vendorDir: vendorDir,
	}
}

// Install устанавливает все зависимости
func (i *Installer) Install(composerJSON *composer.ComposerJSON, dev bool) (*composer.ComposerLock, error) {
	fmt.Println("📦 Resolving dependencies...")

	// Объединяем обычные и dev зависимости
	requirements := make(map[string]string)
	for name, version := range composerJSON.Require {
		requirements[name] = version
	}
	if dev {
		for name, version := range composerJSON.RequireDev {
			requirements[name] = version
		}
	}

	// Разрешаем зависимости
	packages, err := i.resolver.Resolve(requirements)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	fmt.Printf("✅ Resolved %d packages\n\n", len(packages))

	// Создаем vendor директорию
	if err := os.MkdirAll(i.vendorDir, 0755); err != nil {
		return nil, err
	}

	// Устанавливаем пакеты параллельно
	fmt.Println("⬇️  Downloading and installing packages...")
	fmt.Println()

	// Выводим список пакетов, которые будем устанавливать
	for _, pkg := range packages {
		version := pkg.Version
		if pkg.Info.Dist != nil && pkg.Info.Dist.Reference != "" && pkg.Info.Dist.Reference != pkg.Version {
			ref := pkg.Info.Dist.Reference
			if len(ref) > 8 {
				ref = ref[:8]
			}
			version = fmt.Sprintf("%s (%s)", pkg.Version, ref)
		}
		fmt.Printf("  📦 %-40s %s\n", pkg.Name, version)
	}
	fmt.Println()

	var wg sync.WaitGroup
	errors := make(chan error, len(packages))
	lockedPackages := make(chan *composer.LockedPackage, len(packages))

	bar := progressbar.Default(int64(len(packages)), "Installing")

	for _, pkg := range packages {
		wg.Add(1)
		go func(pkg *resolver.Package) {
			defer wg.Done()
			defer bar.Add(1)

			locked, err := i.installPackage(pkg)
			if err != nil {
				errors <- fmt.Errorf("failed to install %s: %w", pkg.Name, err)
				return
			}
			lockedPackages <- locked
		}(pkg)
	}

	wg.Wait()
	close(errors)
	close(lockedPackages)
	bar.Finish()

	// Проверяем ошибки
	if len(errors) > 0 {
		return nil, <-errors
	}

	// Собираем locked пакеты
	var locked []composer.LockedPackage
	for pkg := range lockedPackages {
		locked = append(locked, *pkg)
	}

	// Создаем composer.lock
	contentHash := i.calculateContentHash(composerJSON)
	lock := composer.NewComposerLock(contentHash)
	lock.Packages = locked

	fmt.Println("\n✅ All packages installed successfully!")

	return lock, nil
}

// installPackage устанавливает один пакет
func (i *Installer) installPackage(pkg *resolver.Package) (*composer.LockedPackage, error) {
	// Проверяем, есть ли dist
	if pkg.Info.Dist == nil || pkg.Info.Dist.URL == "" {
		return nil, fmt.Errorf("no distribution URL for package %s", pkg.Name)
	}

	// Загружаем пакет
	data, err := i.client.DownloadPackage(pkg.Info.Dist.URL)
	if err != nil {
		return nil, err
	}

	// Проверяем shasum если есть
	if pkg.Info.Dist.Shasum != "" {
		hash := sha256.Sum256(data)
		actualSum := hex.EncodeToString(hash[:])
		if actualSum != pkg.Info.Dist.Shasum {
			return nil, fmt.Errorf("shasum mismatch for %s", pkg.Name)
		}
	}

	// Распаковываем zip
	if err := i.extractZip(data, pkg.Name); err != nil {
		return nil, err
	}

	// Создаем LockedPackage
	locked := &composer.LockedPackage{
		Name:        pkg.Name,
		Version:     pkg.Version,
		Source:      convertSource(pkg.Info.Source),
		Dist:        convertDist(pkg.Info.Dist),
		Require:     map[string]string(pkg.Info.Require),
		RequireDev:  map[string]string(pkg.Info.RequireDev),
		Type:        pkg.Info.Type,
		Autoload:    convertAutoload(pkg.Info.Autoload),
		License:     pkg.Info.License,
		Authors:     convertAuthors(pkg.Info.Authors),
		Description: pkg.Info.Description,
		Homepage:    pkg.Info.Homepage,
		Keywords:    pkg.Info.Keywords,
		Time:        pkg.Info.Time,
		Support:     pkg.Info.Support,
		Funding:     []map[string]string(pkg.Info.Funding),
	}

	return locked, nil
}

// extractZip распаковывает zip архив в vendor
func (i *Installer) extractZip(data []byte, packageName string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	// Путь для установки пакета
	targetDir := filepath.Join(i.vendorDir, packageName)

	for _, file := range reader.File {
		// Пропускаем корневую директорию в архиве
		parts := strings.Split(file.Name, "/")
		if len(parts) < 2 {
			continue
		}
		relativePath := strings.Join(parts[1:], "/")
		if relativePath == "" {
			continue
		}

		targetPath := filepath.Join(targetDir, relativePath)

		if file.FileInfo().IsDir() {
			os.MkdirAll(targetPath, file.Mode())
			continue
		}

		// Создаем директории
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		// Извлекаем файл
		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// calculateContentHash вычисляет хеш для composer.json
func (i *Installer) calculateContentHash(composerJSON *composer.ComposerJSON) string {
	// Упрощенная версия - в реальности Composer использует более сложный алгоритм
	data := fmt.Sprintf("%v%v", composerJSON.Require, composerJSON.RequireDev)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:12]
}

// Вспомогательные функции конвертации

func convertSource(src *packagist.Source) *composer.Source {
	if src == nil {
		return nil
	}
	return &composer.Source{
		Type:      src.Type,
		URL:       src.URL,
		Reference: src.Reference,
	}
}

func convertDist(dist *packagist.Dist) *composer.Dist {
	if dist == nil {
		return nil
	}
	return &composer.Dist{
		Type:      dist.Type,
		URL:       dist.URL,
		Reference: dist.Reference,
		Shasum:    dist.Shasum,
	}
}

func convertAutoload(autoload packagist.AutoloadConfig) composer.AutoloadConfig {
	config := composer.AutoloadConfig{}

	if psr4, ok := autoload["psr-4"].(map[string]interface{}); ok {
		config.PSR4 = psr4
	}
	if psr0, ok := autoload["psr-0"].(map[string]interface{}); ok {
		config.PSR0 = psr0
	}

	return config
}

func convertAuthors(authors []packagist.Author) []composer.Author {
	result := make([]composer.Author, len(authors))
	for i, a := range authors {
		result[i] = composer.Author{
			Name:     a.Name,
			Email:    a.Email,
			Homepage: a.Homepage,
			Role:     a.Role,
		}
	}
	return result
}
