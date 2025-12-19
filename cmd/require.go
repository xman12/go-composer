package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xman12/go-composer/pkg/autoload"
	"github.com/xman12/go-composer/pkg/composer"
	"github.com/xman12/go-composer/pkg/installer"
)

var (
	requireDev bool
)

var requireCmd = &cobra.Command{
	Use:   "require [packages...]",
	Short: "Add new packages to composer.json and install them",
	Long: `Adds one or more packages to composer.json and installs them.
Usage: go-composer require vendor/package:^1.0`,
	Args: cobra.MinimumNArgs(1),
	RunE: runRequire,
}

func init() {
	requireCmd.Flags().BoolVar(&requireDev, "dev", false, "add to require-dev")
	requireCmd.Flags().BoolVar(&noAutoload, "no-autoloader", false, "skip autoloader generation")
	rootCmd.AddCommand(requireCmd)
}

func runRequire(cmd *cobra.Command, args []string) error {
	// Меняем рабочую директорию если указано
	if workDir != "." {
		if err := os.Chdir(workDir); err != nil {
			return fmt.Errorf("failed to change directory: %w", err)
		}
	}

	composerJSONPath := "composer.json"
	composerLockPathFile := "composer.lock"
	composerLockGoPathFile := "go-composer.lock"
	composerLock := ""
	vendorDir := "vendor"

	fmt.Println("🚀 go-composer - Adding packages")
	fmt.Println()

	// Загружаем или создаем composer.json
	var composerJSON *composer.ComposerJSON
	if _, err := os.Stat(composerJSONPath); os.IsNotExist(err) {
		fmt.Println("📝 Creating new composer.json...")
		composerJSON = &composer.ComposerJSON{
			Require:    make(map[string]string),
			RequireDev: make(map[string]string),
		}
	} else {
		var err error
		composerJSON, err = composer.LoadComposerJSON(composerJSONPath)
		if err != nil {
			return fmt.Errorf("failed to load composer.json: %w", err)
		}
	}

	// Инициализируем map'ы если nil
	if composerJSON.Require == nil {
		composerJSON.Require = make(map[string]string)
	}
	if composerJSON.RequireDev == nil {
		composerJSON.RequireDev = make(map[string]string)
	}

	// Парсим и добавляем пакеты
	for _, pkg := range args {
		parts := strings.SplitN(pkg, ":", 2)
		packageName := parts[0]
		version := "*"
		if len(parts) == 2 {
			version = parts[1]
		}

		if requireDev {
			composerJSON.RequireDev[packageName] = version
			fmt.Printf("➕ Adding %s:%s to require-dev\n", packageName, version)
		} else {
			composerJSON.Require[packageName] = version
			fmt.Printf("➕ Adding %s:%s to require\n", packageName, version)
		}
	}

	// Сохраняем composer.json
	if err := composerJSON.Save(composerJSONPath); err != nil {
		return fmt.Errorf("failed to save composer.json: %w", err)
	}
	fmt.Println("✅ composer.json updated")
	fmt.Println()

	// Создаем installer
	inst := installer.NewInstaller(vendorDir)

	// Устанавливаем зависимости
	lock, err := inst.Install(composerJSON, true)
	if err != nil {
		return err
	}

	if _, err := os.Stat(composerLockGoPathFile); err == nil {
		composerLock = composerLockGoPathFile
	} else if _, err := os.Stat(composerLockPathFile); err == nil {
		composerLock = composerLockPathFile
	}

	// Сохраняем  *.lock
	if err := lock.Save(composerLock); err != nil {
		return fmt.Errorf("failed to save lock: %w", err)
	}

	// Генерируем autoload
	if !noAutoload {
		gen := autoload.NewGenerator(vendorDir)
		if err := gen.Generate(lock, composerJSON); err != nil {
			return fmt.Errorf("failed to generate autoload: %w", err)
		}
	}

	fmt.Println()
	fmt.Println("🎉 Packages installed successfully!")
	return nil
}
