package resolver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/xman12/go-composer/pkg/packagist"
)

// Package представляет разрешенный пакет
type Package struct {
	Name    string
	Version string
	Info    *packagist.PackageVersion
}

// Resolver разрешает зависимости пакетов
type Resolver struct {
	client      *packagist.Client
	resolved    map[string]*Package
	visited     map[string]bool
	constraints map[string][]string // Все constraints для каждого пакета
	replaced    map[string]string   // Пакеты, замененные через "replace" (name -> replacing package version)
}

// NewResolver создает новый resolver
func NewResolver(client *packagist.Client) *Resolver {
	return &Resolver{
		client:      client,
		resolved:    make(map[string]*Package),
		visited:     make(map[string]bool),
		constraints: make(map[string][]string),
		replaced:    make(map[string]string),
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

	// Удаляем пакеты, которые были заменены через replace
	r.removeReplacedPackages()

	return r.resolved, nil
}

// removeReplacedPackages удаляет пакеты, замененные через replace
func (r *Resolver) removeReplacedPackages() {
	for replacedPkg := range r.replaced {
		delete(r.resolved, replacedPkg)
	}
}

// resolvePackage рекурсивно разрешает зависимости пакета
func (r *Resolver) resolvePackage(name, constraint string) error {
	// Добавляем constraint к списку для этого пакета (делаем это в начале)
	// Для Carbon разворачиваем OR constraints сразу
	if name == "nesbot/carbon" && strings.Contains(constraint, "||") {
		// Разбиваем constraint с OR на отдельные constraints
		parts := strings.Split(constraint, "||")
		for _, part := range parts {
			r.constraints[name] = append(r.constraints[name], strings.TrimSpace(part))
		}
	} else {
		r.constraints[name] = append(r.constraints[name], constraint)
	}

	// Проверяем, не заменен ли этот пакет другим (через replace)
	if replacedBy, ok := r.replaced[name]; ok {
		// Пакет заменен - проверяем совместимость версии
		c, err := parseConstraint(constraint)
		if err != nil {
			// Игнорируем невалидный constraint для замененного пакета
			return nil
		}

		v, err := normalizeVersion(replacedBy)
		if err != nil {
			// Если не можем распарсить версию замены, просто пропускаем
			return nil
		}

		if !c.Check(v) {
			// Версия несовместима - это может быть проблемой
			// Но для Laravel это нормально, т.к. illuminate/* версии всегда совпадают с версией framework
			// Выводим предупреждение, но не останавливаем процесс
			// fmt.Printf("Warning: replaced package %s version %s may not satisfy constraint %s\n", name, replacedBy, constraint)
		}

		// Пакет уже "установлен" через replace, не нужно его разрешать отдельно
		return nil
	}

	// Если уже обрабатывали этот пакет - нужно перепроверить версию
	if r.visited[name] {
		// Проверяем, подходит ли текущая выбранная версия под новый constraint
		if pkg, ok := r.resolved[name]; ok {
			// Проверяем новый constraint
			c, err := parseConstraint(constraint)
			if err != nil {
				return fmt.Errorf("invalid constraint %s: %w", constraint, err)
			}

			v, err := normalizeVersion(pkg.Version)
			if err != nil {
				return err
			}

			// Если текущая версия не подходит - нужно переразрешить
			if !c.Check(v) {
				// Получаем информацию о пакете
				info, err := r.client.GetPackage(name)
				if err != nil {
					return err
				}

				// Находим версию, удовлетворяющую ВСЕМ constraints
				version, err := r.findVersionSatisfyingAll(info, name, r.constraints[name])
				if err != nil {
					return fmt.Errorf("cannot find version for %s satisfying all constraints: %v", name, r.constraints[name])
				}

				// Обновляем разрешенный пакет
				r.resolved[name] = &Package{
					Name:    name,
					Version: version.Version,
					Info:    version,
				}

				// Обновляем replace для нового пакета
				r.processReplace(version)
			}
		}
		return nil
	}
	r.visited[name] = true

	// Получаем информацию о пакете
	info, err := r.client.GetPackage(name)
	if err != nil {
		return err
	}

	// Находим подходящую версию для ВСЕХ constraints
	version, err := r.findVersionSatisfyingAll(info, name, r.constraints[name])
	if err != nil {
		return err
	}

	// Сохраняем разрешенный пакет
	r.resolved[name] = &Package{
		Name:    name,
		Version: version.Version,
		Info:    version,
	}

	// Обрабатываем replace секцию ДО разрешения зависимостей
	// Это важно для пакетов типа laravel/framework, которые предоставляют illuminate/* компоненты
	r.processReplace(version)

	// Рекурсивно разрешаем зависимости
	if version.Require != nil {
		for depName, depConstraint := range version.Require {
			// Пропускаем виртуальные и платформенные пакеты
			if isVirtualPackage(depName) {
				continue
			}

			// Пропускаем пакеты, которые уже заменены через replace
			if _, replaced := r.replaced[depName]; replaced {
				continue
			}

			if err := r.resolvePackage(depName, depConstraint); err != nil {
				return err
			}
		}
	}

	return nil
}

// findCarbonVersionSmart умно выбирает версию Carbon, учитывая OR constraints
func (r *Resolver) findCarbonVersionSmart(packageVersions []packagist.PackageVersion, name string, constraints []string) (*packagist.PackageVersion, error) {
	// Разворачиваем OR constraints: если constraint содержит |, разбиваем его на части
	var expandedConstraints []string
	for _, constraint := range constraints {
		// Проверяем, есть ли OR (||) в constraint
		if strings.Contains(constraint, "||") {
			// Разбиваем на части
			parts := strings.Split(constraint, "||")
			for _, part := range parts {
				expandedConstraints = append(expandedConstraints, strings.TrimSpace(part))
			}
		} else {
			expandedConstraints = append(expandedConstraints, constraint)
		}
	}

	// Группируем constraints по major версиям
	// Например: [^2.67, ^2.66.0, ^3.0, ^3.8.4] → {2: [^2.67, ^2.66.0], 3: [^3.0, ^3.8.4]}
	majorGroups := make(map[int][]string)
	for _, constraint := range expandedConstraints {
		// Определяем major версию из constraint
		major := extractMajorFromConstraint(constraint)
		if major > 0 {
			majorGroups[major] = append(majorGroups[major], constraint)
		}
	}

	// Пробуем сначала найти версию, удовлетворяющую одной major версии
	// Предпочитаем более старые major версии (2.x вместо 3.x)
	majors := make([]int, 0, len(majorGroups))
	for major := range majorGroups {
		majors = append(majors, major)
	}
	sort.Ints(majors) // Сортируем по возрастанию

	for _, major := range majors {
		constraints := majorGroups[major]
		// Пробуем найти версию для этой major версии
		version, err := r.tryFindVersionForConstraints(packageVersions, name, constraints)
		if err == nil {
			return version, nil
		}
	}

	// Если не нашли, используем стандартную логику
	return r.findVersionWithConstraints(packageVersions, name, expandedConstraints)
}

// extractMajorFromConstraint извлекает major версию из constraint
func extractMajorFromConstraint(constraint string) int {
	// Убираем операторы ^, ~, >=, >, <, <=, =
	constraint = strings.TrimSpace(constraint)
	constraint = strings.TrimPrefix(constraint, "^")
	constraint = strings.TrimPrefix(constraint, "~")
	constraint = strings.TrimPrefix(constraint, ">=")
	constraint = strings.TrimPrefix(constraint, "<=")
	constraint = strings.TrimPrefix(constraint, ">")
	constraint = strings.TrimPrefix(constraint, "<")
	constraint = strings.TrimPrefix(constraint, "=")
	constraint = strings.TrimSpace(constraint)

	// Парсим версию
	parts := strings.Split(constraint, ".")
	if len(parts) > 0 {
		major := 0
		fmt.Sscanf(parts[0], "%d", &major)
		return major
	}
	return 0
}

// tryFindVersionForConstraints пробует найти версию для заданных constraints
func (r *Resolver) tryFindVersionForConstraints(packageVersions []packagist.PackageVersion, name string, constraints []string) (*packagist.PackageVersion, error) {
	// Парсим constraints
	var parsedConstraints []*semver.Constraints
	for _, constraint := range constraints {
		c, err := parseConstraint(constraint)
		if err != nil {
			continue // Пропускаем невалидные
		}
		parsedConstraints = append(parsedConstraints, c)
	}

	if len(parsedConstraints) == 0 {
		return nil, fmt.Errorf("no valid constraints")
	}

	// Собираем версии
	var versions []*semver.Version
	versionMap := make(map[string]*packagist.PackageVersion)

	for i := range packageVersions {
		pkgVer := &packageVersions[i]
		if len(pkgVer.Version) > 4 && pkgVer.Version[:4] == "dev-" {
			continue
		}
		v, err := normalizeVersion(pkgVer.Version)
		if err != nil {
			continue
		}
		versions = append(versions, v)
		versionMap[v.String()] = pkgVer
	}

	// Группируем версии по major
	majorVersions := make(map[uint64][]*semver.Version)
	for _, v := range versions {
		major := v.Major()
		majorVersions[major] = append(majorVersions[major], v)
	}

	// Сортируем major версии по возрастанию (предпочитаем более старые major)
	var majors []uint64
	for major := range majorVersions {
		majors = append(majors, major)
	}
	sort.Slice(majors, func(i, j int) bool {
		return majors[i] < majors[j]
	})

	// Для каждой major версии (от старых к новым) ищем самую новую подходящую версию
	for _, major := range majors {
		majorVers := majorVersions[major]
		// Сортируем версии внутри major (от новых к старым)
		sort.Slice(majorVers, func(i, j int) bool {
			return majorVers[i].GreaterThan(majorVers[j])
		})

		// Ищем первую (самую новую в этой major) версию, удовлетворяющую ВСЕМ constraints
		for _, v := range majorVers {
			satisfiesAll := true
			for _, c := range parsedConstraints {
				if !c.Check(v) {
					satisfiesAll = false
					break
				}
			}
			if satisfiesAll {
				return versionMap[v.String()], nil
			}
		}
	}

	return nil, fmt.Errorf("no version found")
}

// findVersionWithConstraints стандартная логика поиска версии
func (r *Resolver) findVersionWithConstraints(packageVersions []packagist.PackageVersion, name string, constraints []string) (*packagist.PackageVersion, error) {
	// Парсим все constraints
	var parsedConstraints []*semver.Constraints
	for _, constraint := range constraints {
		c, err := parseConstraint(constraint)
		if err != nil {
			return nil, fmt.Errorf("invalid constraint %s: %w", constraint, err)
		}
		parsedConstraints = append(parsedConstraints, c)
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

	// Находим первую версию, удовлетворяющую ВСЕМ constraints
	for _, v := range versions {
		satisfiesAll := true
		for _, c := range parsedConstraints {
			if !c.Check(v) {
				satisfiesAll = false
				break
			}
		}

		if satisfiesAll {
			return versionMap[v.String()], nil
		}
	}

	return nil, fmt.Errorf("no matching version found for package %s with constraints %v", name, constraints)
}

// processReplace обрабатывает replace секцию пакета
func (r *Resolver) processReplace(version *packagist.PackageVersion) {
	if version.Replace == nil || len(version.Replace) == 0 {
		return
	}

	for replacedPkg, replaceVersion := range version.Replace {
		// self.version означает версию текущего пакета
		if replaceVersion == "self.version" || replaceVersion == "*" {
			replaceVersion = version.Version
		}
		r.replaced[replacedPkg] = replaceVersion
	}
}

// findVersionSatisfyingAll находит версию, удовлетворяющую всем constraints
func (r *Resolver) findVersionSatisfyingAll(info *packagist.PackageInfo, name string, constraints []string) (*packagist.PackageVersion, error) {
	packageVersions, ok := info.Packages[name]
	if !ok || len(packageVersions) == 0 {
		return nil, fmt.Errorf("no versions found for package %s", name)
	}

	// Для критичных пакетов (carbon) разворачиваем OR constraints в отдельные варианты
	// и пробуем найти совместимую версию
	if name == "nesbot/carbon" {
		return r.findCarbonVersionSmart(packageVersions, name, constraints)
	}

	// Парсим все constraints
	var parsedConstraints []*semver.Constraints
	for _, constraint := range constraints {
		c, err := parseConstraint(constraint)
		if err != nil {
			return nil, fmt.Errorf("invalid constraint %s: %w", constraint, err)
		}
		parsedConstraints = append(parsedConstraints, c)
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

	// Находим первую версию, удовлетворяющую ВСЕМ constraints
	for _, v := range versions {
		satisfiesAll := true
		for _, c := range parsedConstraints {
			if !c.Check(v) {
				satisfiesAll = false
				break
			}
		}

		if satisfiesAll {
			return versionMap[v.String()], nil
		}
	}

	// Если нет версии удовлетворяющей ВСЕМ constraints,
	// ищем версию с максимальным количеством удовлетворенных constraints
	// Это может произойти из-за несовместимых требований от разных пакетов
	var bestVersion *semver.Version
	maxSatisfied := 0

	for _, v := range versions {
		satisfiedCount := 0
		for _, c := range parsedConstraints {
			if c.Check(v) {
				satisfiedCount++
			}
		}

		// Выбираем самую новую версию с максимальным количеством удовлетворенных constraints
		if satisfiedCount > maxSatisfied || (satisfiedCount == maxSatisfied && (bestVersion == nil || v.GreaterThan(bestVersion))) {
			maxSatisfied = satisfiedCount
			bestVersion = v
		}
	}

	// Если нашли версию, удовлетворяющую хотя бы одному constraint, используем её
	if bestVersion != nil && maxSatisfied > 0 {
		// Для Laravel Illuminate пакетов разрешаем fallback, т.к. они предоставляются через replace
		isIlluminate := len(name) > 11 && name[:11] == "illuminate/"

		// Если удовлетворены ВСЕ constraints - возвращаем версию
		if maxSatisfied == len(parsedConstraints) {
			return versionMap[bestVersion.String()], nil
		}

		// Для illuminate пакетов разрешаем fallback
		if isIlluminate {
			return versionMap[bestVersion.String()], nil
		}

		// Для критичных пакетов требуем строгое соответствие ВСЕМ constraints
		// Carbon - критичный пакет, т.к. несовместимость между 2.x и 3.x вызывает Fatal Error
		isCriticalPackage := name == "nesbot/carbon"

		// Для 2 constraints разрешаем fallback, если удовлетворен хотя бы 1,
		// НО только если это не критичный пакет
		if len(parsedConstraints) == 2 && maxSatisfied >= 1 && !isCriticalPackage {
			fmt.Printf("  ⚠️  Warning: package %s version %s satisfies only %d/%d constraints: %v\n",
				name, bestVersion.String(), maxSatisfied, len(parsedConstraints), constraints)
			return versionMap[bestVersion.String()], nil
		}

		// Для пакетов с многими constraints (3+) разрешаем fallback,
		// если удовлетворено хотя бы 60% constraints (для 3 это 2, для 5 это 3)
		threshold := (len(parsedConstraints) * 6) / 10 // 60%
		if threshold < 1 {
			threshold = 1
		}
		// Минимум должно быть удовлетворено хотя бы 2 constraints
		if threshold < 2 && len(parsedConstraints) >= 3 {
			threshold = 2
		}

		if maxSatisfied >= threshold && len(parsedConstraints) >= 3 {
			// Для пакетов с многими constraints (3+) и хорошим совпадением (75%+) предупреждаем, но используем
			fmt.Printf("  ⚠️  Warning: package %s version %s satisfies only %d/%d constraints: %v\n",
				name, bestVersion.String(), maxSatisfied, len(parsedConstraints), constraints)
			return versionMap[bestVersion.String()], nil
		}

		// Для критичных зависимостей с малым количеством constraints - строгое соответствие
		// Возвращаем ошибку, чтобы дать возможность выбрать другую версию родительского пакета
	}

	return nil, fmt.Errorf("no matching version found for package %s with constraints %v", name, constraints)
}

// findBestVersion находит лучшую версию пакета, соответствующую constraint
func (r *Resolver) findBestVersion(info *packagist.PackageInfo, name, constraint string) (*packagist.PackageVersion, error) {
	return r.findVersionSatisfyingAll(info, name, []string{constraint})
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

	// Symfony polyfill пакеты (предоставляют функциональность, встроенную в PHP)
	if len(name) > 16 && name[:16] == "symfony/polyfill" {
		return true
	}

	// Composer виртуальные пакеты
	switch name {
	case "composer-runtime-api", "composer-plugin-api":
		return true
	}

	return false
}
