package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/xman12/go-composer/pkg/autoload"
	"github.com/xman12/go-composer/pkg/composer"
	"github.com/xman12/go-composer/pkg/installer"
	"github.com/xman12/go-composer/pkg/scripts"
)

var (
	noDev        bool
	noAutoload   bool
	newLock      bool
	forceNewLock bool
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
	installCmd.Flags().BoolVar(&newLock, "new-lock", true, "create go-composer.lock file")
	installCmd.Flags().BoolVar(&forceNewLock, "force-new-lock", false, "force new go-composer.lock file")
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
	composerLockPathFile := "composer.lock"
	composerLockGoPathFile := "go-composer.lock"
	composerLock := ""
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

	if newLock {
		if _, err := os.Stat(composerLockGoPathFile); err == nil {
			composerLock = composerLockGoPathFile
		} else if _, err := os.Stat(composerLockPathFile); err == nil {
			composerLock = composerLockPathFile
		}
	} else {
		if _, err := os.Stat(composerLockPathFile); err == nil {
			composerLock = composerLockPathFile
		}
	}

	if forceNewLock {
		composerLock = ""
	}

	// Проверяем наличие composer.lock
	if composerLock != "" {
		// Lock файл существует - устанавливаем напрямую из него
		fmt.Println("📋 Found composer.lock, installing from lock file...")
		lock, err = composer.LoadComposerLock(composerLock)
		if err != nil {
			return fmt.Errorf("failed to load composer.lock: %w", err)
		}

		// Устанавливаем напрямую из lock без resolve через Packagist
		if err := inst.InstallFromLock(lock, !noDev); err != nil {
			return fmt.Errorf("failed to install packages: %w", err)
		}
	} else {
		// Lock файла нет - делаем resolve и устанавливаем, но устанавливаем в go-composer.lock
		// так как в данный момент файлы не имеют совместимость и чтобы не ломать совместимость
		// делаем так
		fmt.Println("📋 No lock file found, resolving dependencies...")
		lock, err = inst.Install(composerJSON, !noDev)
		if err != nil {
			return err
		}

		if forceNewLock {
			composerLock = composerLockGoPathFile
		} else {
			composerLock = composerLockPathFile
		}

		if newLock {
			composerLock = composerLockGoPathFile
		}

		// Сохраняем lock
		if err := lock.Save(composerLock); err != nil {
			return fmt.Errorf("failed to save .lock: %w", err)
		}
		fmt.Println("✅ .lock created")
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
