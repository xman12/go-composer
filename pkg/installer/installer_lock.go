package installer

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/schollz/progressbar/v3"
	"github.com/xman12/go-composer/pkg/composer"
)

// InstallFromLock устанавливает пакеты напрямую из composer.lock без resolve
func (i *Installer) InstallFromLock(lock *composer.ComposerLock, dev bool) error {
	fmt.Printf("✅ Found %d packages in composer.lock\n\n", len(lock.Packages))

	// Создаем vendor директорию
	if err := os.MkdirAll(i.vendorDir, 0755); err != nil {
		return err
	}

	// Устанавливаем пакеты параллельно
	fmt.Println("⬇️  Downloading and installing packages from lock file...")
	fmt.Println()

	// Выводим список пакетов
	for _, pkg := range lock.Packages {
		version := pkg.Version
		if pkg.Dist != nil && pkg.Dist.Reference != "" && pkg.Dist.Reference != pkg.Version {
			ref := pkg.Dist.Reference
			if len(ref) > 8 {
				ref = ref[:8]
			}
			version = fmt.Sprintf("%s (%s)", pkg.Version, ref)
		}
		fmt.Printf("  📦 %-40s %s\n", pkg.Name, version)
	}
	fmt.Println()

	if dev {
		// Установка дев пакетов
		for _, pkgDev := range lock.PackagesDev {
			version := pkgDev.Version
			if pkgDev.Dist != nil && pkgDev.Dist.Reference != "" && pkgDev.Dist.Reference != pkgDev.Version {
				ref := pkgDev.Dist.Reference
				if len(ref) > 8 {
					ref = ref[:8]
				}
				version = fmt.Sprintf("%s (%s)", pkgDev.Version, ref)
			}
			fmt.Printf("  📦 %-40s %s\n", pkgDev.Name, version)
		}
		fmt.Println()
	}

	var wg sync.WaitGroup
	errors := make(chan error, len(lock.Packages))

	bar := progressbar.Default(int64(len(lock.Packages)), "Installing")

	for _, pkg := range lock.Packages {
		wg.Add(1)
		go func(pkg composer.LockedPackage) {
			defer wg.Done()
			defer bar.Add(1)

			if err := i.installLockedPackage(pkg); err != nil {
				errors <- fmt.Errorf("failed to install %s: %w", pkg.Name, err)
				return
			}
		}(pkg)
	}

	if dev {
		for _, pkg := range lock.PackagesDev {
			wg.Add(1)
			go func(pkg composer.LockedPackage) {
				defer wg.Done()
				defer bar.Add(1)

				if err := i.installLockedPackage(pkg); err != nil {
					errors <- fmt.Errorf("failed to install %s: %w", pkg.Name, err)
					return
				}
			}(pkg)
		}
	}

	wg.Wait()
	close(errors)
	bar.Finish()

	// Проверяем ошибки
	select {
	case err := <-errors:
		if err != nil {
			return err
		}
	default:
	}

	fmt.Println("\n✅ All packages installed successfully!")
	return nil
}

// installLockedPackage устанавливает пакет из composer.lock
func (i *Installer) installLockedPackage(pkg composer.LockedPackage) error {
	// Используем информацию напрямую из composer.lock
	if pkg.Dist == nil || pkg.Dist.URL == "" {
		return fmt.Errorf("no dist URL for package %s", pkg.Name)
	}

	packageDir := filepath.Join(i.vendorDir, pkg.Name)

	// Проверяем, установлен ли уже пакет
	if _, err := os.Stat(packageDir); err == nil {
		return nil // Уже установлен
	}

	// Скачиваем архив
	resp, err := http.Get(pkg.Dist.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download package: HTTP %d", resp.StatusCode)
	}

	// Сохраняем во временный файл
	tmpFile, err := os.CreateTemp("", "go-composer-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return err
	}

	// Проверяем SHA (если есть)
	if pkg.Dist.Shasum != "" {
		tmpFile.Seek(0, 0)
		hash := sha256.New()
		if _, err := io.Copy(hash, tmpFile); err != nil {
			return err
		}
		actualHash := hex.EncodeToString(hash.Sum(nil))
		if actualHash != pkg.Dist.Shasum {
			return fmt.Errorf("checksum mismatch for %s", pkg.Name)
		}
	}

	// Распаковываем
	tmpFile.Seek(0, 0)
	stat, _ := tmpFile.Stat()
	zipReader, err := zip.NewReader(tmpFile, stat.Size())
	if err != nil {
		return err
	}

	// Создаем директорию для пакета
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		return err
	}

	// Извлекаем файлы
	for _, file := range zipReader.File {
		// Пропускаем первую директорию (обычно это vendor-package-version/)
		parts := strings.Split(file.Name, "/")
		if len(parts) <= 1 {
			continue
		}
		relativePath := strings.Join(parts[1:], "/")
		if relativePath == "" {
			continue
		}

		targetPath := filepath.Join(packageDir, relativePath)

		if file.FileInfo().IsDir() {
			os.MkdirAll(targetPath, file.Mode())
			continue
		}

		// Создаем родительскую директорию
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		// Копируем файл
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
