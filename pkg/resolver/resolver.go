package resolver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/aleksandrbelysev/go-composer/pkg/packagist"
)

// Package представляет разрешенный пакет
type Package struct {
	Name    string
	Version string
	Info    *packagist.PackageVersion
}

// Resolver разрешает зависимости пакетов
type Resolver struct {
	client   *packagist.Client
	resolved map[string]*Package
	visited  map[string]bool
}

// NewResolver создает новый resolver
func NewResolver(client *packagist.Client) *Resolver {
	return &Resolver{
		client:   client,
		resolved: make(map[string]*Package),
		visited:  make(map[string]bool),
	}
}

// Resolve разрешает все зависимости
func (r *Resolver) Resolve(requirements map[string]string) (map[string]*Package, error) {
	for name, constraint := range requirements {
		// Пропускаем виртуальные и платформенные пакеты
		if isVirtualPackage(name) {
			continue
		}

		if err := r.resolvePackage(name, constraint); err != nil {
			return nil, fmt.Errorf("failed to resolve %s: %w", name, err)
		}
	}

	return r.resolved, nil
}

// resolvePackage рекурсивно разрешает зависимости пакета
func (r *Resolver) resolvePackage(name, constraint string) error {
	// Проверяем, не обрабатывали ли мы уже этот пакет
	if r.visited[name] {
		return nil
	}
	r.visited[name] = true

	// Получаем информацию о пакете
	info, err := r.client.GetPackage(name)
	if err != nil {
		return err
	}

	// Находим подходящую версию
	version, err := r.findBestVersion(info, name, constraint)
	if err != nil {
		return err
	}

	// Сохраняем разрешенный пакет
	r.resolved[name] = &Package{
		Name:    name,
		Version: version.Version,
		Info:    version,
	}

	// Рекурсивно разрешаем зависимости
	if version.Require != nil {
		for depName, depConstraint := range version.Require {
			// Пропускаем виртуальные и платформенные пакеты
			if isVirtualPackage(depName) {
				continue
			}

			if err := r.resolvePackage(depName, depConstraint); err != nil {
				return err
			}
		}
	}

	return nil
}

// findBestVersion находит лучшую версию пакета, соответствующую constraint
func (r *Resolver) findBestVersion(info *packagist.PackageInfo, name, constraint string) (*packagist.PackageVersion, error) {
	packageVersions, ok := info.Packages[name]
	if !ok || len(packageVersions) == 0 {
		return nil, fmt.Errorf("no versions found for package %s", name)
	}

	// Парсим constraint
	c, err := parseConstraint(constraint)
	if err != nil {
		return nil, fmt.Errorf("invalid constraint %s: %w", constraint, err)
	}

	// Собираем все версии
	var versions []*semver.Version
	versionMap := make(map[string]*packagist.PackageVersion)

	for i := range packageVersions {
		pkgVer := &packageVersions[i]
		
		// Пропускаем dev-версии
		if len(pkgVer.Version) > 4 && pkgVer.Version[:4] == "dev-" {
			continue
		}

		v, err := normalizeVersion(pkgVer.Version)
		if err != nil {
			continue // Пропускаем невалидные версии
		}

		versions = append(versions, v)
		versionMap[v.String()] = pkgVer
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no valid versions found for package %s", name)
	}

	// Сортируем версии (от новых к старым)
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].GreaterThan(versions[j])
	})

	// Находим первую подходящую версию
	for _, v := range versions {
		if c.Check(v) {
			return versionMap[v.String()], nil
		}
	}

	return nil, fmt.Errorf("no matching version found for package %s with constraint %s", name, constraint)
}

// parseConstraint парсит constraint в semver Constraint
func parseConstraint(constraint string) (*semver.Constraints, error) {
	// Обрабатываем специальные случаи
	switch constraint {
	case "*":
		constraint = ">=0.0.0"
	case "":
		constraint = ">=0.0.0"
	}

	// Нормализуем пробелы
	constraint = strings.TrimSpace(constraint)
	
	// Composer использует | для ИЛИ, а semver использует ||
	// Сначала заменяем все варианты с пробелами
	constraint = strings.ReplaceAll(constraint, " || ", "||")
	constraint = strings.ReplaceAll(constraint, " | ", "||")
	
	// Затем заменяем одиночные | на ||, но только если это не часть ||
	var result strings.Builder
	for i := 0; i < len(constraint); i++ {
		if constraint[i] == '|' {
			// Проверяем, не является ли это уже частью ||
			if i+1 < len(constraint) && constraint[i+1] == '|' {
				result.WriteByte('|')
				result.WriteByte('|')
				i++ // пропускаем следующий |
			} else {
				result.WriteString("||")
			}
		} else {
			result.WriteByte(constraint[i])
		}
	}
	constraint = result.String()
	
	// Добавляем пробелы вокруг || для совместимости
	constraint = strings.ReplaceAll(constraint, "||", " || ")
	
	// Убираем множественные пробелы
	for strings.Contains(constraint, "  ") {
		constraint = strings.ReplaceAll(constraint, "  ", " ")
	}
	constraint = strings.TrimSpace(constraint)
	
	// Преобразуем caret (^) и tilde (~) в понятный semver формат
	// ^ означает совместимые изменения (^1.2.3 -> >=1.2.3 <2.0.0)
	// ~ означает патч-уровень (~1.2.3 -> >=1.2.3 <1.3.0)
	
	return semver.NewConstraint(constraint)
}

// normalizeVersion нормализует версию для semver
func normalizeVersion(version string) (*semver.Version, error) {
	// Убираем префикс 'v' если есть
	if len(version) > 0 && version[0] == 'v' {
		version = version[1:]
	}

	return semver.NewVersion(version)
}

// isVirtualPackage проверяет, является ли пакет виртуальным/платформенным
func isVirtualPackage(name string) bool {
	// PHP и его расширения
	if name == "php" {
		return true
	}
	
	// PHP расширения (ext-mbstring, ext-pdo и т.д.)
	if len(name) > 4 && name[:4] == "ext-" {
		return true
	}
	
	// Системные библиотеки (lib-curl, lib-openssl и т.д.)
	if len(name) > 4 && name[:4] == "lib-" {
		return true
	}
	
	// Composer виртуальные пакеты
	switch name {
	case "composer-runtime-api", "composer-plugin-api":
		return true
	}
	
	return false
}

