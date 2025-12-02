package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aleksandrbelysev/go-composer/pkg/autoload"
	"github.com/aleksandrbelysev/go-composer/pkg/composer"
	"github.com/aleksandrbelysev/go-composer/pkg/installer"
	"github.com/aleksandrbelysev/go-composer/pkg/scripts"
	"github.com/spf13/cobra"
)

var (
	noDev      bool
	noAutoload bool
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install dependencies from composer.lock",
	Long: `Reads composer.lock (or composer.json if lock doesn't exist)
and installs all dependencies into vendor/ directory.`,
	RunE: runInstall,
}

func init() {
	installCmd.Flags().BoolVar(&noDev, "no-dev", false, "skip dev dependencies")
	installCmd.Flags().BoolVar(&noAutoload, "no-autoloader", false, "skip autoloader generation")
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	// Меняем рабочую директорию если указано
	if workDir != "." {
		if err := os.Chdir(workDir); err != nil {
			return fmt.Errorf("failed to change directory: %w", err)
		}
	}

	composerJSONPath := "composer.json"
	composerLockPath := "composer.lock"
	vendorDir := "vendor"

	// Проверяем наличие composer.json
	if _, err := os.Stat(composerJSONPath); os.IsNotExist(err) {
		return fmt.Errorf("composer.json not found in current directory")
	}

	fmt.Println("🚀 go-composer - Fast PHP Dependency Manager")
	fmt.Println()

	// Загружаем composer.json
	composerJSON, err := composer.LoadComposerJSON(composerJSONPath)
	if err != nil {
		return fmt.Errorf("failed to load composer.json: %w", err)
	}

	// Создаем executor для скриптов
	projectRoot, _ := filepath.Abs(".")
	scriptExecutor := scripts.NewExecutor(projectRoot, composerJSON)

	// 1️⃣ Выполняем pre-install-cmd скрипты (ПЕРЕД установкой пакетов)
	if err := scriptExecutor.Execute(scripts.EventPreInstallCmd); err != nil {
		fmt.Printf("⚠️  Warning: pre-install-cmd failed: %v\n", err)
	}

	// Создаем installer
	inst := installer.NewInstaller(vendorDir)

	var lock *composer.ComposerLock

	// Проверяем наличие composer.lock
	if _, err := os.Stat(composerLockPath); err == nil {
		// Lock файл существует - устанавливаем напрямую из него
		fmt.Println("📋 Found composer.lock, installing from lock file...")
		lock, err = composer.LoadComposerLock(composerLockPath)
		if err != nil {
			return fmt.Errorf("failed to load composer.lock: %w", err)
		}

		// Устанавливаем напрямую из lock без resolve через Packagist
		if err := inst.InstallFromLock(lock, !noDev); err != nil {
			return fmt.Errorf("failed to install packages: %w", err)
		}
	} else {
		// Lock файла нет - делаем resolve и устанавливаем
		fmt.Println("📋 No lock file found, resolving dependencies...")
		lock, err = inst.Install(composerJSON, !noDev)
		if err != nil {
			return err
		}

		// Сохраняем composer.lock
		if err := lock.Save(composerLockPath); err != nil {
			return fmt.Errorf("failed to save composer.lock: %w", err)
		}
		fmt.Println("✅ composer.lock created")
	}

	// Генерируем autoload
	if !noAutoload {
		// 2️⃣ Выполняем pre-autoload-dump скрипты (ПЕРЕД генерацией autoload)
		if err := scriptExecutor.Execute(scripts.EventPreAutoloadDump); err != nil {
			fmt.Printf("⚠️  Warning: pre-autoload-dump failed: %v\n", err)
		}

		gen := autoload.NewGenerator(vendorDir)
		if err := gen.Generate(lock, composerJSON); err != nil {
			return fmt.Errorf("failed to generate autoload: %w", err)
		}

		// 3️⃣ Выполняем post-autoload-dump скрипты (ПОСЛЕ генерации autoload)
		if err := scriptExecutor.Execute(scripts.EventPostAutoloadDump); err != nil {
			fmt.Printf("⚠️  Warning: post-autoload-dump failed: %v\n", err)
		}
	}

	// 4️⃣ Выполняем post-install-cmd скрипты (ПОСЛЕ установки пакетов)
	if err := scriptExecutor.Execute(scripts.EventPostInstallCmd); err != nil {
		fmt.Printf("⚠️  Warning: post-install-cmd failed: %v\n", err)
	}

	fmt.Println()
	fmt.Println("🎉 Installation complete!")
	return nil
}
