package autoload

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xman12/go-composer/pkg/composer"
)

// Generator генерирует autoload файлы
type Generator struct {
	vendorDir string
}

// NewGenerator создает новый генератор
func NewGenerator(vendorDir string) *Generator {
	// Приводим vendorDir к абсолютному пути
	absVendorDir, err := filepath.Abs(vendorDir)
	if err != nil {
		// В случае ошибки используем как есть
		absVendorDir = vendorDir
	}

	return &Generator{
		vendorDir: absVendorDir,
	}
}

// Generate генерирует autoload.php
func (g *Generator) Generate(lock *composer.ComposerLock, composerJSON *composer.ComposerJSON) error {
	fmt.Println("🔧 Generating autoload files...")

	// Собираем все PSR-4 и PSR-0 mappings
	psr4Map := make(map[string][]string)
	psr0Map := make(map[string][]string)
	classmapDirs := []string{}
	files := []string{}

	// Из composer.json проекта (относительно корня проекта)
	projectRoot := filepath.Dir(g.vendorDir)
	g.addAutoloadConfig(composerJSON.Autoload, psr4Map, psr0Map, &classmapDirs, &files, projectRoot)
	g.addAutoloadConfig(composerJSON.AutoloadDev, psr4Map, psr0Map, &classmapDirs, &files, projectRoot)

	// Из всех установленных пакетов
	for _, pkg := range lock.Packages {
		packageDir := filepath.Join(g.vendorDir, pkg.Name)

		// Читаем composer.json пакета напрямую
		packageComposerPath := filepath.Join(packageDir, "composer.json")
		if pkgComposer, err := composer.LoadComposerJSON(packageComposerPath); err == nil {
			g.addAutoloadConfig(pkgComposer.Autoload, psr4Map, psr0Map, &classmapDirs, &files, packageDir)
		} else {
			// Fallback на данные из lock файла
			g.addAutoloadConfig(pkg.Autoload, psr4Map, psr0Map, &classmapDirs, &files, packageDir)
		}
	}

	// Из дев пакетов
	for _, pkg := range lock.PackagesDev {
		packageDir := filepath.Join(g.vendorDir, pkg.Name)

		// Читаем composer.json пакета напрямую
		packageComposerPath := filepath.Join(packageDir, "composer.json")
		if pkgComposer, err := composer.LoadComposerJSON(packageComposerPath); err == nil {
			g.addAutoloadConfig(pkgComposer.Autoload, psr4Map, psr0Map, &classmapDirs, &files, packageDir)
		} else {
			// Fallback на данные из lock файла
			g.addAutoloadConfig(pkg.Autoload, psr4Map, psr0Map, &classmapDirs, &files, packageDir)
		}
	}

	// Создаем autoload.php
	if err := g.generateAutoloadPHP(psr4Map, psr0Map, classmapDirs, files); err != nil {
		return err
	}

	// Создаем ClassLoader.php
	if err := g.generateClassLoader(); err != nil {
		return err
	}

	// Создаем autoload_runtime.php (только для Symfony проектов)
	if err := g.generateRuntimeAutoload(lock); err != nil {
		return err
	}

	// Создаем vendor/composer/installed.json для Composer 2 совместимости
	if err := g.generateInstalledJson(lock); err != nil {
		return err
	}

	// Создаем vendor/composer/InstalledVersions.php для Composer 2
	if err := g.generateInstalledVersions(); err != nil {
		return err
	}

	// Создаем vendor/composer/platform_check.php для Composer 2
	if err := g.generatePlatformCheck(); err != nil {
		return err
	}

	// Генерируем autoload_classmap.php
	if err := g.generateClassmap(lock); err != nil {
		return err
	}

	fmt.Println("✅ Autoload files generated")
	return nil
}

// patchFile заменяет текст в файле
func (g *Generator) patchFile(filePath, oldText, newText string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	content := string(data)
	if !strings.Contains(content, oldText) {
		return fmt.Errorf("pattern not found")
	}

	newContent := strings.ReplaceAll(content, oldText, newText)
	return os.WriteFile(filePath, []byte(newContent), 0644)
}

// addAutoloadConfig добавляет конфигурацию автозагрузки
func (g *Generator) addAutoloadConfig(
	config composer.AutoloadConfig,
	psr4Map, psr0Map map[string][]string,
	classmapDirs, files *[]string,
	baseDir string,
) {
	// PSR-4
	if config.PSR4 != nil {
		for ns, pathInterface := range config.PSR4 {
			namespace := ns
			var paths []string

			switch v := pathInterface.(type) {
			case string:
				paths = []string{v}
			case []interface{}:
				for _, p := range v {
					if str, ok := p.(string); ok {
						paths = append(paths, str)
					}
				}
			}

			for _, path := range paths {
				var fullPath string
				if baseDir != "" {
					fullPath = filepath.Join(baseDir, path)
				} else {
					fullPath = path
				}
				psr4Map[namespace] = append(psr4Map[namespace], fullPath)
			}
		}
	}

	// PSR-0
	if config.PSR0 != nil {
		for ns, pathInterface := range config.PSR0 {
			namespace := ns
			var paths []string

			switch v := pathInterface.(type) {
			case string:
				paths = []string{v}
			case []interface{}:
				for _, p := range v {
					if str, ok := p.(string); ok {
						paths = append(paths, str)
					}
				}
			}

			for _, path := range paths {
				fullPath := path
				if baseDir != "" {
					fullPath = filepath.Join(baseDir, path)
				}
				psr0Map[namespace] = append(psr0Map[namespace], fullPath)
			}
		}
	}

	// Classmap
	for _, dir := range config.Classmap {
		fullPath := dir
		if baseDir != "" {
			fullPath = filepath.Join(baseDir, dir)
		}
		*classmapDirs = append(*classmapDirs, fullPath)
	}

	// Files
	for _, file := range config.Files {
		fullPath := file
		if baseDir != "" {
			fullPath = filepath.Join(baseDir, file)
		}
		*files = append(*files, fullPath)
	}
}

// generateAutoloadPHP генерирует autoload.php
func (g *Generator) generateAutoloadPHP(
	psr4Map, psr0Map map[string][]string,
	classmapDirs, files []string,
) error {
	autoloadPath := filepath.Join(g.vendorDir, "autoload.php")

	content := `<?php

// autoload.php @generated by go-composer

require_once __DIR__ . '/ClassLoader.php';

// Load Composer classes for Composer 2 compatibility
if (file_exists(__DIR__ . '/composer/InstalledVersions.php')) {
    require_once __DIR__ . '/composer/InstalledVersions.php';
}
if (file_exists(__DIR__ . '/composer/platform_check.php')) {
    require_once __DIR__ . '/composer/platform_check.php';
}

$loader = new \Composer\Autoload\ClassLoader();

// Load classmap
if (file_exists(__DIR__ . '/composer/autoload_classmap.php')) {
    $classMap = require __DIR__ . '/composer/autoload_classmap.php';
    if ($classMap) {
        $loader->addClassMap($classMap);
    }
}

// PSR-4 autoloading
`

	// PSR-4
	for namespace, paths := range psr4Map {
		for _, path := range paths {
			relPath := g.makeRelativePath(path)
			content += fmt.Sprintf("$loader->addPsr4('%s', __DIR__ . '%s');\n",
				strings.ReplaceAll(namespace, "\\", "\\\\"), relPath)
		}
	}

	content += "\n// PSR-0 autoloading\n"

	// PSR-0
	for namespace, paths := range psr0Map {
		for _, path := range paths {
			relPath := g.makeRelativePath(path)
			content += fmt.Sprintf("$loader->add('%s', __DIR__ . '%s');\n",
				strings.ReplaceAll(namespace, "\\", "\\\\"), relPath)
		}
	}

	content += "\n$loader->register();\n\n"

	// Автоматически находим и подключаем bootstrap файлы
	bootstrapFiles := g.findBootstrapFiles()
	if len(bootstrapFiles) > 0 {
		content += "// Bootstrap files\n"
		for _, file := range bootstrapFiles {
			relPath := g.makeRelativePath(file)
			content += fmt.Sprintf("if (file_exists(__DIR__ . '%s')) { require_once __DIR__ . '%s'; }\n", relPath, relPath)
		}
		content += "\n"
	}

	// Files из autoload
	if len(files) > 0 {
		content += "// Autoload files\n"
		for _, file := range files {
			relPath := g.makeRelativePath(file)
			content += fmt.Sprintf("if (file_exists(__DIR__ . '%s')) { require_once __DIR__ . '%s'; }\n", relPath, relPath)
		}
	}

	content += "\nreturn $loader;\n"

	return os.WriteFile(autoloadPath, []byte(content), 0644)
}

// generateClassLoader генерирует ClassLoader.php
func (g *Generator) generateClassLoader() error {
	classLoaderPath := filepath.Join(g.vendorDir, "ClassLoader.php")

	content := `<?php

// ClassLoader.php @generated by go-composer

namespace Composer\Autoload;

class ClassLoader
{
    private $prefixesPsr4 = [];
    private $prefixesPsr0 = [];
    private $classMap = [];

    public function addPsr4($prefix, $baseDir)
    {
        // Нормализуем базовую директорию
        $baseDir = rtrim($baseDir, '/\\') . '/';

        if (!isset($this->prefixesPsr4[$prefix])) {
            $this->prefixesPsr4[$prefix] = [];
        }
        $this->prefixesPsr4[$prefix][] = $baseDir;
    }

    public function add($prefix, $baseDir)
    {
        // Нормализуем базовую директорию
        $baseDir = rtrim($baseDir, '/\\') . '/';

        if (!isset($this->prefixesPsr0[$prefix])) {
            $this->prefixesPsr0[$prefix] = [];
        }
        $this->prefixesPsr0[$prefix][] = $baseDir;
    }

    public function register()
    {
        spl_autoload_register([$this, 'loadClass']);
    }

    public function loadClass($class)
    {
        if ($file = $this->findFile($class)) {
            require $file;
            return true;
        }
        return false;
    }

    public function findFile($class)
    {
        // Проверяем в classmap
        if (isset($this->classMap[$class])) {
            return $this->classMap[$class];
        }

        // PSR-4
        if ($file = $this->findFilePsr4($class)) {
            if (file_exists($file)) {
                return $file;
            }
        }

        // PSR-0
        if ($file = $this->findFilePsr0($class)) {
            if (file_exists($file)) {
                return $file;
            }
        }

        return false;
    }

    private function findFilePsr4($class)
    {
        // Проверяем каждый зарегистрированный namespace
        foreach ($this->prefixesPsr4 as $prefix => $dirs) {
            // Проверяем, начинается ли класс с этого prefix
            $len = strlen($prefix);
            if (strncmp($prefix, $class, $len) === 0) {
                // Получаем относительный путь класса (без prefix)
                $relativeClass = substr($class, $len);

                // Проверяем каждую директорию для этого prefix
                foreach ($dirs as $dir) {
                    // Формируем полный путь к файлу
                    $file = $dir . str_replace('\\', '/', $relativeClass) . '.php';
                    if (file_exists($file)) {
                        return $file;
                    }
                }
            }
        }

        return false;
    }

    private function findFilePsr0($class)
    {
        $pos = strrpos($class, '\\');

        // Полное имя класса с namespace
        $logicalPath = str_replace('\\', '/', $class) . '.php';

        foreach ($this->prefixesPsr0 as $prefix => $dirs) {
            if (strpos($class, $prefix) === 0) {
                foreach ($dirs as $dir) {
                    $file = $dir . $logicalPath;
                    if (file_exists($file)) {
                        return $file;
                    }
                }
            }
        }

        return false;
    }

    // Методы для получения конфигурации (используются Symfony)
    public function getPrefixes()
    {
        return $this->prefixesPsr0;
    }

    public function getPrefixesPsr4()
    {
        return $this->prefixesPsr4;
    }

    public function getClassMap()
    {
        return $this->classMap;
    }

    public function addClassMap(array $classMap)
    {
        if ($this->classMap) {
            $this->classMap = array_merge($this->classMap, $classMap);
        } else {
            $this->classMap = $classMap;
        }
    }

    public function getFallbackDirs()
    {
        return array();
    }

    public function getFallbackDirsPsr4()
    {
        return array();
    }
}
`

	return os.WriteFile(classLoaderPath, []byte(content), 0644)
}

// makeRelativePath делает путь относительным к vendor директории для PHP
func (g *Generator) makeRelativePath(path string) string {
	var absPath string

	// Приводим путь к абсолютному виду
	if filepath.IsAbs(path) {
		absPath = path
	} else {
		// Убираем префикс vendor/ если есть
		path = strings.TrimPrefix(path, "vendor/")
		path = strings.TrimPrefix(path, "vendor\\")
		// Делаем абсолютным относительно vendorDir
		absPath = filepath.Join(g.vendorDir, path)
	}

	// Получаем относительный путь от vendor к целевому пути
	rel, err := filepath.Rel(g.vendorDir, absPath)
	if err != nil {
		// В случае ошибки возвращаем как есть
		return "/" + filepath.ToSlash(absPath)
	}

	// Если это текущая директория
	if rel == "." {
		return ""
	}

	// Преобразуем в Unix-style слэши для PHP
	rel = filepath.ToSlash(rel)

	// Добавляем / в начало для PHP пути
	if !strings.HasPrefix(rel, "/") {
		rel = "/" + rel
	}

	return rel
}

// findBootstrapFiles ищет bootstrap файлы в установленных пакетах
func (g *Generator) findBootstrapFiles() []string {
	var bootstrapFiles []string

	// Список известных bootstrap файлов
	bootstrapPatterns := []string{
		"symfony/polyfill-*/bootstrap.php",
		"symfony/deprecation-contracts/function.php",
		"symfony/string/Resources/functions.php",
	}

	for _, pattern := range bootstrapPatterns {
		matches, _ := filepath.Glob(filepath.Join(g.vendorDir, pattern))
		for _, match := range matches {
			bootstrapFiles = append(bootstrapFiles, match)
		}
	}

	return bootstrapFiles
}

// generateRuntimeAutoload генерирует autoload_runtime.php для Symfony Runtime
func (g *Generator) generateRuntimeAutoload(lock *composer.ComposerLock) error {
	// Проверяем, используется ли Symfony Runtime
	isSymfonyRuntime := false
	for _, pkg := range lock.Packages {
		if pkg.Name == "symfony/runtime" {
			isSymfonyRuntime = true
			break
		}
	}

	if !isSymfonyRuntime {
		// Если Symfony Runtime не используется, не создаём файл
		return nil
	}

	runtimeAutoloadPath := filepath.Join(g.vendorDir, "autoload_runtime.php")

	content := `<?php

// autoload_runtime.php @generated by go-composer

if (true === (require_once __DIR__.'/autoload.php') || empty($_SERVER['SCRIPT_FILENAME'])) {
    return;
}

$app = require $_SERVER['SCRIPT_FILENAME'];

if (!is_object($app)) {
    throw new TypeError(sprintf('Invalid return value: callable object expected, "%s" returned from "%s".', get_debug_type($app), $_SERVER['SCRIPT_FILENAME']));
}

$runtime = $_SERVER['APP_RUNTIME'] ?? $_ENV['APP_RUNTIME'] ?? 'Symfony\\Component\\Runtime\\SymfonyRuntime';
$runtime = new $runtime(($_SERVER['APP_RUNTIME_OPTIONS'] ?? $_ENV['APP_RUNTIME_OPTIONS'] ?? []) + [
        'project_dir' => dirname(__DIR__, 1),
    ]);

[$app, $args] = $runtime
    ->getResolver($app)
    ->resolve();

$app = $app(...$args);

exit(
$runtime
    ->getRunner($app)
    ->run()
);
`

	return os.WriteFile(runtimeAutoloadPath, []byte(content), 0644)
}

// generateInstalledJson создает vendor/composer/installed.json для Composer 2
func (g *Generator) generateInstalledJson(lock *composer.ComposerLock) error {
	composerDir := filepath.Join(g.vendorDir, "composer")
	if err := os.MkdirAll(composerDir, 0755); err != nil {
		return err
	}

	installedPath := filepath.Join(composerDir, "installed.json")

	// Формируем список установленных пакетов
	type InstalledPackage struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Type    string `json:"type,omitempty"`
	}

	var packages []InstalledPackage
	for _, pkg := range lock.Packages {
		packages = append(packages, InstalledPackage{
			Name:    pkg.Name,
			Version: pkg.Version,
			Type:    pkg.Type,
		})
	}

	installed := map[string]interface{}{
		"packages":          packages,
		"dev":               true,
		"dev-package-names": []string{},
	}

	data, err := json.MarshalIndent(installed, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(installedPath, data, 0644)
}

// generateInstalledVersions создает vendor/composer/InstalledVersions.php для Composer 2
func (g *Generator) generateInstalledVersions() error {
	composerDir := filepath.Join(g.vendorDir, "composer")
	versionsPath := filepath.Join(composerDir, "InstalledVersions.php")

	// Упрощенная версия для совместимости с Symfony
	content := `<?php

// InstalledVersions.php @generated by go-composer

namespace Composer;

use Composer\Autoload\ClassLoader;

class InstalledVersions
{
    private static $installed;
    private static $canGetVendors;
    private static $installedByVendor = array();

    public static function getInstalledPackages()
    {
        $packages = array();
        if (file_exists(__DIR__ . '/installed.json')) {
            $installed = json_decode(file_get_contents(__DIR__ . '/installed.json'), true);
            foreach ($installed['packages'] as $package) {
                $packages[] = $package['name'];
            }
        }
        return $packages;
    }

    public static function isInstalled($packageName, $includeDevRequirements = true)
    {
        return in_array($packageName, self::getInstalledPackages(), true);
    }

    public static function getVersion($packageName)
    {
        if (file_exists(__DIR__ . '/installed.json')) {
            $installed = json_decode(file_get_contents(__DIR__ . '/installed.json'), true);
            foreach ($installed['packages'] as $package) {
                if ($package['name'] === $packageName) {
                    return $package['version'];
                }
            }
        }
        return null;
    }

    public static function getVersionRanges($packageName)
    {
        return self::getVersion($packageName);
    }

    public static function getAllRawData()
    {
        if (file_exists(__DIR__ . '/installed.json')) {
            return array(
                'root' => array('install_path' => dirname(__DIR__, 2)),
                'versions' => json_decode(file_get_contents(__DIR__ . '/installed.json'), true),
            );
        }
        return array();
    }
}
`

	return os.WriteFile(versionsPath, []byte(content), 0644)
}

// generatePlatformCheck создает vendor/composer/platform_check.php для Composer 2
func (g *Generator) generatePlatformCheck() error {
	composerDir := filepath.Join(g.vendorDir, "composer")
	platformPath := filepath.Join(composerDir, "platform_check.php")

	content := `<?php

// platform_check.php @generated by go-composer
// This file is used by Symfony to detect Composer 2

$issues = array();

if (!(PHP_VERSION_ID >= 70205)) {
    $issues[] = 'Your Composer dependencies require a PHP version ">= 7.2.5". You are running ' . PHP_VERSION . '.';
}

if ($issues) {
    if (!headers_sent()) {
        header('HTTP/1.1 500 Internal Server Error');
    }
    if (!ini_get('display_errors')) {
        if (PHP_SAPI === 'cli' || PHP_SAPI === 'phpdbg') {
            fwrite(STDERR, 'Composer detected issues in your platform:' . PHP_EOL.PHP_EOL . implode(PHP_EOL, $issues) . PHP_EOL.PHP_EOL);
        } elseif (!headers_sent()) {
            echo 'Composer detected issues in your platform:' . PHP_EOL.PHP_EOL . str_replace('You are running '.PHP_VERSION.'.', '', implode(PHP_EOL, $issues)) . PHP_EOL.PHP_EOL;
        }
    }
    trigger_error(
        'Composer detected issues in your platform: ' . implode(' ', $issues),
        E_USER_ERROR
    );
}
`

	return os.WriteFile(platformPath, []byte(content), 0644)
}

// generateClassmap генерирует autoload_classmap.php
func (g *Generator) generateClassmap(lock *composer.ComposerLock) error {
	composerDir := filepath.Join(g.vendorDir, "composer")
	classmapPath := filepath.Join(composerDir, "autoload_classmap.php")

	// Собираем classmap из всех пакетов
	classMap := make(map[string]string)

	// Проходим по всем пакетам
	for _, pkg := range lock.Packages {
		packageDir := filepath.Join(g.vendorDir, pkg.Name)

		// Читаем composer.json пакета
		composerPath := filepath.Join(packageDir, "composer.json")
		data, err := os.ReadFile(composerPath)
		if err != nil {
			continue // Пропускаем если файл не найден
		}

		var pkgComposer composer.ComposerJSON
		if err := json.Unmarshal(data, &pkgComposer); err != nil {
			continue
		}

		// Обрабатываем classmap из autoload
		g.processClassmapDirs(pkgComposer.Autoload, packageDir, classMap)
		g.processClassmapDirs(pkgComposer.AutoloadDev, packageDir, classMap)
	}

	// Генерируем PHP файл с classmap
	content := "<?php\n\n// autoload_classmap.php @generated by go-composer\n\n"
	content += "return array(\n"

	for className, filePath := range classMap {
		// Делаем путь относительным к vendor
		relPath := g.makeRelativePath(filePath)
		content += fmt.Sprintf("    '%s' => __DIR__ . '%s',\n",
			strings.ReplaceAll(className, "\\", "\\\\"), relPath)
	}

	content += ");\n"

	return os.WriteFile(classmapPath, []byte(content), 0644)
}

// processClassmapDirs обрабатывает classmap директории из autoload конфигурации
func (g *Generator) processClassmapDirs(config composer.AutoloadConfig, baseDir string, classMap map[string]string) {
	if len(config.Classmap) == 0 {
		return
	}

	for _, dir := range config.Classmap {
		fullPath := filepath.Join(baseDir, dir)
		// Сканируем директорию на наличие PHP файлов
		g.scanClassmapDir(fullPath, classMap)
	}
}

// scanClassmapDir рекурсивно сканирует директорию и находит PHP классы
func (g *Generator) scanClassmapDir(dir string, classMap map[string]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			// Рекурсивно сканируем поддиректории
			g.scanClassmapDir(fullPath, classMap)
		} else if strings.HasSuffix(entry.Name(), ".php") {
			// Пытаемся извлечь имя класса из файла
			className := g.extractClassNameFromFile(fullPath)
			if className != "" {
				classMap[className] = fullPath
			}
		}
	}
}

// extractClassNameFromFile извлекает fully qualified имя класса из PHP файла
func (g *Generator) extractClassNameFromFile(filePath string) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	content := string(data)

	// Ищем namespace
	var namespace string
	namespaceRegex := regexp.MustCompile(`namespace\s+([a-zA-Z0-9_\\]+)\s*;`)
	if matches := namespaceRegex.FindStringSubmatch(content); len(matches) > 1 {
		namespace = matches[1]
	}

	// Ищем class, interface или trait
	classRegex := regexp.MustCompile(`(?:class|interface|trait)\s+([a-zA-Z0-9_]+)`)
	if matches := classRegex.FindStringSubmatch(content); len(matches) > 1 {
		className := matches[1]
		if namespace != "" {
			return namespace + "\\" + className
		}
		return className
	}

	return ""
}
