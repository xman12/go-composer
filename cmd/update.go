package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/xman12/go-composer/pkg/autoload"
	"github.com/xman12/go-composer/pkg/composer"
	"github.com/xman12/go-composer/pkg/installer"
)

var updateCmd = &cobra.Command{
	Use:   "update [packages...]",
	Short: "Update dependencies to their latest versions",
	Long: `Updates dependencies to their latest versions according to
composer.json constraints and updates composer.lock file.`,
	RunE: runUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&noDev, "no-dev", false, "skip dev dependencies")
	updateCmd.Flags().BoolVar(&noAutoload, "no-autoloader", false, "skip autoloader generation")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
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

	fmt.Println("🚀 go-composer - Updating dependencies")
	fmt.Println()

	// Загружаем composer.json
	composerJSON, err := composer.LoadComposerJSON(composerJSONPath)
	if err != nil {
		return fmt.Errorf("failed to load composer.json: %w", err)
	}

	// Создаем installer
	inst := installer.NewInstaller(vendorDir)

	// Разрешаем и устанавливаем зависимости
	lock, err := inst.Install(composerJSON, !noDev)
	if err != nil {
		return err
	}

	if _, err := os.Stat(composerLockGoPathFile); err == nil {
		composerLock = composerLockGoPathFile
	} else if _, err := os.Stat(composerLockPathFile); err == nil {
		composerLock = composerLockPathFile
	}

	// Сохраняем composer.lock
	if err := lock.Save(composerLock); err != nil {
		return fmt.Errorf("failed to save lock: %w", err)
	}
	fmt.Println("✅ composer.lock updated")

	// Генерируем autoload
	if !noAutoload {
		gen := autoload.NewGenerator(vendorDir)
		if err := gen.Generate(lock, composerJSON); err != nil {
			return fmt.Errorf("failed to generate autoload: %w", err)
		}
	}

	fmt.Println()
	fmt.Println("🎉 Update complete!")
	return nil
}
