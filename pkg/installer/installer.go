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

	"github.com/schollz/progressbar/v3"
	"github.com/xman12/go-composer/pkg/composer"
	"github.com/xman12/go-composer/pkg/packagist"
	"github.com/xman12/go-composer/pkg/resolver"
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

	// Сначала разрешаем основные зависимости
	mainPackages, err := i.resolver.Resolve(composerJSON.Require)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Сохраняем имена основных пакетов для разделения
	mainPackageNames := make(map[string]bool)
	for name := range mainPackages {
		mainPackageNames[name] = true
	}

	// Затем разрешаем dev зависимости (если нужно)
	var devPackages map[string]*resolver.Package
	if dev && len(composerJSON.RequireDev) > 0 {
		// Создаем новый resolver для dev зависимостей
		devResolver := resolver.NewResolver(i.client)

		// Объединяем все требования (основные + dev)
		allRequirements := make(map[string]string)
		for name, version := range composerJSON.Require {
			allRequirements[name] = version
		}
		for name, version := range composerJSON.RequireDev {
			allRequirements[name] = version
		}

		allPackages, err := devResolver.Resolve(allRequirements)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve dev dependencies: %w", err)
		}

		// Выделяем только dev пакеты (которых нет в основных)
		devPackages = make(map[string]*resolver.Package)
		for name, pkg := range allPackages {
			if !mainPackageNames[name] {
				devPackages[name] = pkg
			}
		}
	}

	totalPackages := len(mainPackages) + len(devPackages)
	fmt.Printf("✅ Resolved %d packages (%d main + %d dev)\n\n", totalPackages, len(mainPackages), len(devPackages))

	// Создаем vendor директорию
	if err := os.MkdirAll(i.vendorDir, 0755); err != nil {
		return nil, err
	}

	// Объединяем все пакеты для установки
	allPackages := make(map[string]*resolver.Package)
	for name, pkg := range mainPackages {
		allPackages[name] = pkg
	}
	for name, pkg := range devPackages {
		allPackages[name] = pkg
	}

	// Устанавливаем пакеты параллельно
	fmt.Println("⬇️  Downloading and installing packages...")
	fmt.Println()

	// Выводим список пакетов, которые будем устанавливать
	for _, pkg := range allPackages {
		version := pkg.Version
		if pkg.Info.Dist.Dist != nil && pkg.Info.Dist.Dist.Reference != "" && pkg.Info.Dist.Dist.Reference != pkg.Version {
			ref := pkg.Info.Dist.Dist.Reference
			if len(ref) > 8 {
				ref = ref[:8]
			}
			version = fmt.Sprintf("%s (%s)", pkg.Version, ref)
		}
		devMarker := ""
		if devPackages != nil {
			if _, isDev := devPackages[pkg.Name]; isDev {
				devMarker = " [dev]"
			}
		}
		fmt.Printf("  📦 %-40s %s%s\n", pkg.Name, version, devMarker)
	}
	fmt.Println()

	var wg sync.WaitGroup
	errors := make(chan error, len(allPackages))
	type lockedResult struct {
		pkg   *composer.LockedPackage
		isDev bool
	}
	lockedPackages := make(chan lockedResult, len(allPackages))

	bar := progressbar.Default(int64(len(allPackages)), "Installing")

	for _, pkg := range allPackages {
		wg.Add(1)
		isDev := false
		if devPackages != nil {
			_, isDev = devPackages[pkg.Name]
		}
		go func(pkg *resolver.Package, isDev bool) {
			defer wg.Done()
			defer bar.Add(1)

			locked, err := i.installPackage(pkg)
			if err != nil {
				errors <- fmt.Errorf("failed to install %s: %w", pkg.Name, err)
				return
			}
			lockedPackages <- lockedResult{pkg: locked, isDev: isDev}
		}(pkg, isDev)
	}

	wg.Wait()
	close(errors)
	close(lockedPackages)
	bar.Finish()

	// Проверяем ошибки
	if len(errors) > 0 {
		return nil, <-errors
	}

	// Собираем locked пакеты с разделением на main и dev
	var lockedMain []composer.LockedPackage
	var lockedDev []composer.LockedPackage
	for result := range lockedPackages {
		if result.isDev {
			lockedDev = append(lockedDev, *result.pkg)
		} else {
			lockedMain = append(lockedMain, *result.pkg)
		}
	}

	// Создаем composer.lock
	contentHash := i.calculateContentHash(composerJSON)
	lock := composer.NewComposerLock(contentHash)
	lock.Packages = lockedMain
	lock.PackagesDev = lockedDev

	fmt.Println("\n✅ All packages installed successfully!")

	return lock, nil
}

// installPackage устанавливает один пакет
func (i *Installer) installPackage(pkg *resolver.Package) (*composer.LockedPackage, error) {
	// Проверяем, есть ли dist
	if pkg.Info.Dist.Dist == nil || pkg.Info.Dist.Dist.URL == "" {
		return nil, fmt.Errorf("no distribution URL for package %s", pkg.Name)
	}

	// Загружаем пакет
	data, err := i.client.DownloadPackage(pkg.Info.Dist.Dist.URL)
	if err != nil {
		return nil, err
	}

	// Проверяем shasum если есть
	if pkg.Info.Dist.Dist.Shasum != "" {
		hash := sha256.Sum256(data)
		actualSum := hex.EncodeToString(hash[:])
		if actualSum != pkg.Info.Dist.Dist.Shasum {
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

func convertDist(flexDist packagist.FlexibleDist) *composer.Dist {
	if flexDist.Dist == nil {
		return nil
	}
	return &composer.Dist{
		Type:      flexDist.Dist.Type,
		URL:       flexDist.Dist.URL,
		Reference: flexDist.Dist.Reference,
		Shasum:    flexDist.Dist.Shasum,
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
