package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xman12/go-composer/pkg/composer"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a basic composer.json file",
	Long:  `Creates a basic composer.json file in the current directory.`,
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Меняем рабочую директорию если указано
	if workDir != "." {
		if err := os.Chdir(workDir); err != nil {
			return fmt.Errorf("failed to change directory: %w", err)
		}
	}

	composerJSONPath := "composer.json"

	// Проверяем, не существует ли уже composer.json
	if _, err := os.Stat(composerJSONPath); err == nil {
		return fmt.Errorf("composer.json already exists")
	}

	fmt.Println("🚀 go-composer - Initialize project")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Спрашиваем информацию о проекте
	fmt.Print("Package name (<vendor>/<name>): ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	fmt.Print("Description: ")
	description, _ := reader.ReadString('\n')
	description = strings.TrimSpace(description)

	fmt.Print("Author name: ")
	authorName, _ := reader.ReadString('\n')
	authorName = strings.TrimSpace(authorName)

	fmt.Print("Author email: ")
	authorEmail, _ := reader.ReadString('\n')
	authorEmail = strings.TrimSpace(authorEmail)

	// Создаем composer.json
	composerJSON := &composer.ComposerJSON{
		Name:        name,
		Description: description,
		Type:        "project",
		Require:     make(map[string]string),
		RequireDev:  make(map[string]string),
	}

	// Добавляем автора если указан
	if authorName != "" {
		composerJSON.Authors = []composer.Author{
			{
				Name:  authorName,
				Email: authorEmail,
			},
		}
	}

	// Добавляем PHP requirement
	composerJSON.Require["php"] = ">=7.4"

	// Сохраняем
	if err := composerJSON.Save(composerJSONPath); err != nil {
		return fmt.Errorf("failed to save composer.json: %w", err)
	}

	fmt.Println()
	fmt.Println("✅ composer.json created successfully!")
	return nil
}
