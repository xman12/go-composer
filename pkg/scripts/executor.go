package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aleksandrbelysev/go-composer/pkg/composer"
)

// Executor выполняет скрипты из composer.json
type Executor struct {
	projectRoot string
	composerJSON *composer.ComposerJSON
}

// NewExecutor создает новый executor для скриптов
func NewExecutor(projectRoot string, composerJSON *composer.ComposerJSON) *Executor {
	return &Executor{
		projectRoot: projectRoot,
		composerJSON: composerJSON,
	}
}

// Execute выполняет скрипты для указанного события
func (e *Executor) Execute(event string) error {
	if e.composerJSON == nil || e.composerJSON.Scripts == nil {
		return nil
	}

	scripts := e.composerJSON.Scripts.GetScripts(event)
	if len(scripts) == 0 {
		return nil
	}

	fmt.Printf("🔧 Running scripts for event: %s\n", event)

	for _, script := range scripts {
		if err := e.executeScript(script); err != nil {
			return fmt.Errorf("failed to execute script '%s': %w", script, err)
		}
	}

	return nil
}

// executeScript выполняет один скрипт
func (e *Executor) executeScript(script string) error {
	// Обрабатываем специальные команды Composer
	if strings.HasPrefix(script, "@") {
		return e.executeComposerCommand(script)
	}

	// Проверяем, это PHP класс::метод?
	if strings.Contains(script, "::") {
		return e.executePHPClassMethod(script)
	}

	// Обычная shell команда
	return e.executeShellCommand(script)
}

// executeComposerCommand выполняет специальные Composer команды (@php, @composer и т.д.)
func (e *Executor) executeComposerCommand(script string) error {
	parts := strings.Fields(script)
	if len(parts) == 0 {
		return nil
	}

	command := parts[0]
	args := parts[1:]

	switch command {
	case "@php":
		// Выполняем PHP скрипт
		return e.executePHP(args)
	case "@composer":
		// Выполняем composer команду (пропускаем, т.к. это рекурсивный вызов)
		fmt.Printf("  ⚠️  Skipping recursive @composer command: %s\n", script)
		return nil
	case "@putenv":
		// Устанавливаем переменную окружения
		if len(args) > 0 {
			parts := strings.SplitN(args[0], "=", 2)
			if len(parts) == 2 {
				os.Setenv(parts[0], parts[1])
			}
		}
		return nil
	default:
		// Неизвестная команда - выполняем как shell
		return e.executeShellCommand(script)
	}
}

// executePHP выполняет PHP команду
func (e *Executor) executePHP(args []string) error {
	if len(args) == 0 {
		return nil
	}

	cmd := exec.Command("php", args...)
	cmd.Dir = e.projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	fmt.Printf("  ▶️  php %s\n", strings.Join(args, " "))
	return cmd.Run()
}

// executePHPClassMethod выполняет PHP класс::метод из Composer scripts
func (e *Executor) executePHPClassMethod(script string) error {
	// Формат: ClassName::methodName
	// Composer передает объект Event в метод, нужно создать mock
	
	vendorAutoload := filepath.Join(e.projectRoot, "vendor", "autoload.php")
	
	// Проверяем существование autoload.php
	if _, err := os.Stat(vendorAutoload); os.IsNotExist(err) {
		// Autoload еще не создан, пропускаем
		fmt.Printf("  ⚠️  Skipping %s (autoload.php not found)\n", script)
		return nil
	}

	// Создаем PHP код с полным mock Event класса
	vendorPath := filepath.Join(e.projectRoot, "vendor")
	
	phpCode := `
		require '` + vendorAutoload + `';
		
		// Создаем namespace и классы Composer, если их нет
		if (!class_exists('Composer\Script\Event', false)) {
			eval('
				namespace Composer\Script {
					class Event {
						private $composer;
						private $io;
						private $name;
						
						public function __construct($name = "post-autoload-dump", $composer = null, $io = null) {
							$this->name = $name;
							$this->composer = $composer;
							$this->io = $io;
						}
						
						public function getComposer() { return $this->composer; }
						public function getIO() { return $this->io; }
						public function getName() { return $this->name; }
						public function getArguments() { return []; }
						public function getFlags() { return []; }
						public function isDevMode() { return false; }
					}
				}
				
				namespace Composer\Config {
					class Config {
						private $vendorDir;
						
						public function __construct($vendorDir) {
							$this->vendorDir = $vendorDir;
						}
						
						public function get($key, $default = null) {
							if ($key === "vendor-dir") {
								return $this->vendorDir;
							}
							return $default;
						}
					}
				}
				
				namespace Composer {
					class Composer {
						private $config;
						
						public function __construct($vendorDir) {
							$this->config = new \Composer\Config\Config($vendorDir);
						}
						
						public function getConfig() { return $this->config; }
					}
				}
			');
		}
		
		$reflection = new ReflectionMethod('` + script + `');
		$params = $reflection->getParameters();
		
		// Если метод требует Event, создаем его
		if (count($params) > 0) {
			$vendorDir = '` + vendorPath + `';
			$composer = new \Composer\Composer($vendorDir);
			$event = new \Composer\Script\Event('post-autoload-dump', $composer, null);
			
			\` + script + `($event);
		} else {
			\` + script + `();
		}
	`
	
	cmd := exec.Command("php", "-r", phpCode)
	cmd.Dir = e.projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	fmt.Printf("  ▶️  %s\n", script)
	return cmd.Run()
}

// executeShellCommand выполняет shell команду
func (e *Executor) executeShellCommand(script string) error {
	// Определяем shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell, "-c", script)
	cmd.Dir = e.projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	// Добавляем путь к vendor/bin в PATH
	vendorBin := filepath.Join(e.projectRoot, "vendor", "bin")
	currentPath := os.Getenv("PATH")
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s:%s", vendorBin, currentPath))

	fmt.Printf("  ▶️  %s\n", script)
	return cmd.Run()
}

// Список всех поддерживаемых событий
const (
	EventPreInstallCmd          = "pre-install-cmd"
	EventPostInstallCmd         = "post-install-cmd"
	EventPreUpdateCmd           = "pre-update-cmd"
	EventPostUpdateCmd          = "post-update-cmd"
	EventPreAutoloadDump        = "pre-autoload-dump"
	EventPostAutoloadDump       = "post-autoload-dump"
	EventPostRootPackageInstall = "post-root-package-install"
	EventPostCreateProjectCmd   = "post-create-project-cmd"
	EventPreArchiveCmd          = "pre-archive-cmd"
	EventPostArchiveCmd         = "post-archive-cmd"
	EventPreStatusCmd           = "pre-status-cmd"
	EventPostStatusCmd          = "post-status-cmd"
	EventPrePackageInstall      = "pre-package-install"
	EventPostPackageInstall     = "post-package-install"
	EventPrePackageUpdate       = "pre-package-update"
	EventPostPackageUpdate      = "post-package-update"
	EventPrePackageUninstall    = "pre-package-uninstall"
	EventPostPackageUninstall   = "post-package-uninstall"
)

