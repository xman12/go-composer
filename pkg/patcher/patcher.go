package patcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Patcher применяет патчи для совместимости пакетов
type Patcher struct {
	vendorDir string
}

// NewPatcher создает новый патчер
func NewPatcher(vendorDir string) *Patcher {
	return &Patcher{
		vendorDir: vendorDir,
	}
}

// ApplyPatches применяет все необходимые патчи
func (p *Patcher) ApplyPatches() error {
	//patches := []struct {
	//	name string
	//	fn   func() error
	//}{
	//	{"Symfony 5.4 PHP 8.1 compatibility", p.patchSymfony54PHP81},
	//}

	//for _, patch := range patches {
	//	if err := patch.fn(); err != nil {
	//		fmt.Printf("⚠️  Warning: Failed to apply patch '%s': %v\n", patch.name, err)
	//	}
	//}

	return nil
}

// patchSymfony54PHP81 исправляет проблемы совместимости Symfony 5.4 с PHP 8.1
func (p *Patcher) patchSymfony54PHP81() error {
	// Исправляем Router::getSubscribedServices()
	routerPath := filepath.Join(p.vendorDir, "symfony/framework-bundle/Routing/Router.php")
	if err := p.patchFile(routerPath,
		"public static function getSubscribedServices()",
		"public static function getSubscribedServices(): array"); err != nil {
		return err
	}

	return nil
}

// patchFile заменяет текст в файле
func (p *Patcher) patchFile(filePath, oldText, newText string) error {
	// Проверяем существование файла
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // Файл не существует, пропускаем
	}

	// Читаем файл
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	content := string(data)

	// Проверяем, нужен ли патч
	if !strings.Contains(content, oldText) {
		return nil // Патч уже применен или не нужен
	}

	// Применяем патч
	newContent := strings.ReplaceAll(content, oldText, newText)

	// Записываем изменения
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return err
	}

	fmt.Printf("✅ Applied patch to %s\n", filepath.Base(filePath))
	return nil
}
