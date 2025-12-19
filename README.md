# go-composer

> A high-performance alternative to Composer for PHP, written in Go.

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![PHP](https://img.shields.io/badge/PHP-7.2+-777BB4?style=flat&logo=php)](https://php.net/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

## ⚠️ Development Status

> **Warning**: This project is currently under active development and is **NOT recommended for production use**.
>
> While go-composer has been successfully tested with various PHP projects (Symfony, Monolog, PHPUnit, etc.), it should be considered **experimental** at this stage. Use at your own risk and thoroughly test in your development environment before considering any production deployment.
> There is currently no full compatibility with composer.lock when create project
> that's why we create our own go-composer.lock, only when don't have composer.lock

##  Features

- ️ **Fast**: 3-5x faster than PHP Composer
-  **Parallel downloads**: Downloads all packages concurrently using goroutines
-  **Packagist compatible**: Works with the standard Packagist API
-  **Drop-in replacement**: Uses the same `composer.json` and `composer.lock` files
-  **No dependencies**: Single binary (~8MB), no PHP required to run
-  **Composer 2 compatible**: Generates `InstalledVersions.php`, `installed.json`, and `platform_check.php`
-  **PSR-4/PSR-0**: Full autoloading support with proper path resolution
-  **Smart bootstrap detection**: Automatically finds and includes polyfill bootstrap files
-  **Progress bars**: Beautiful progress indicators during installation

##  Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/xman12/go-composer.git
cd go-composer

# Build
make build

# Optional: Install globally
sudo make install
```

Or build manually:

```bash
go build -o go-composer .
```

### Usage

```bash
# Install dependencies from composer.lock (or resolve from composer.json)
go-composer install

# Update dependencies to their latest versions
go-composer update

# Add a new package
go-composer require monolog/monolog

# Add a package with specific version
go-composer require symfony/console "^5.0"

# Initialize a new project
go-composer init
```

##  How it works

1. **Reads** `composer.json` from the current directory
2. **Resolves** dependencies using semantic versioning with Packagist API
3. **Downloads** packages from Packagist in parallel using goroutines
4. **Extracts** packages to `vendor/{vendor}/{package}/` directory
5. **Generates** PSR-4/PSR-0 autoloader with correct relative paths
6. **Creates** `go-composer.lock` file with locked versions or using composer.lock if he isset
7. **Generates** Composer 2 compatibility files (`InstalledVersions.php`, etc.)

## 🏗️ Project Structure

```
go-composer/
├── cmd/                    # CLI commands (Cobra)
│   ├── root.go             # Root command and global flags
│   ├── init.go             # Initialize composer.json
│   ├── install.go          # Install dependencies
│   ├── update.go           # Update dependencies
│   └── require.go          # Add new packages
├── pkg/
│   ├── composer/           # composer.json/lock parsing and writing
│   │   ├── composer.go     # JSON structures
│   │   └── lock.go         # Lock file handling
│   ├── packagist/          # Packagist API client
│   │   └── client.go       # Flexible JSON parsing for API responses
│   ├── resolver/           # Dependency resolution
│   │   └── resolver.go     # Semver constraint resolution with OR support
│   ├── installer/          # Package installation
│   │   └── installer.go    # Parallel downloads and extraction
│   └── autoload/           # Autoloader generation
│       └── generator.go    # PSR-4/PSR-0 with bootstrap detection
├── examples/               # Example PHP projects
│   └── simple-monolog/     # Monolog example (3 packages)
├── main.go                 # Entry point
├── Makefile                # Build automation
└── go.mod                  # Go dependencies
```

## Real-World Testing

### Simple Project (Monolog)
```bash
cd examples/simple-monog
./go-composer install
php index.php
```

**Output:**
```
✅ Resolved 3 packages
⬇️  Downloading and installing packages...
Installing 100% |████████████████████| (3/3)
✅ All packages installed successfully!
🔧 Generating autoload files...
✅ Autoload files generated
🎉 Installation complete!
```

## 📊 Performance Comparison

Typical installation times compared to PHP Composer:

| Project | Packages | PHP Composer | go-composer | Speedup |
|---------|----------|--------------|-------------|---------|
| Monolog | 3 | 5-8s | 2s | **3-4x** |
| Symfony App | 36 | 15-20s | 3-5s | **4-5x** |
| Large Project | 80+ | 45-60s | 12-15s | **4-5x** |

*Benchmarked on MacBook Pro M1, 100 Mbps connection*

## Supported Features

### Core Features
- ✅ `composer.json` parsing (require, autoload, authors, etc.)
- ✅ `composer.lock` reading
- ✅ `go-composer.lock` creating
- ✅ Packagist API integration
- ✅ Semver constraint resolution (`^`, `~`, `>=`, `||`, `|`, `*`)
- ✅ Recursive dependency resolution
- ✅ Parallel package downloads
- ✅ SHA-256 checksum verification
- ✅ ZIP archive extraction

### Autoloading
- ✅ PSR-4 autoloading
- ✅ PSR-0 autoloading
- ✅ Correct relative paths (`/../src` for project namespaces)
- ✅ Automatic bootstrap file detection
  - `symfony/polyfill-*/bootstrap.php`
  - `symfony/deprecation-contracts/function.php`
  - `symfony/string/Resources/functions.php`

### Composer 2 Compatibility
- ✅ `vendor/composer/installed.json` - package list
- ✅ `vendor/composer/InstalledVersions.php` - version API
- ✅ `vendor/composer/platform_check.php` - platform checks

### Virtual Packages
- ✅ `php` - PHP version
- ✅ `ext-*` - PHP extensions
- ✅ `lib-*` - system libraries
- ✅ `composer-runtime-api` - Composer runtime
- ✅ `composer-plugin-api` - Composer plugins

### CLI
- ✅ `go-composer init` - interactive project initialization
- ✅ `go-composer install` - install from lock file
- ✅ `go-composer update` - update dependencies
- ✅ `go-composer require` - add new packages
- ✅ Flags: `--no-dev`, `--no-autoloader`, `-v`, `-d`, `new-lock`, `force-new-lock`

Flag `new-lock` using by default = true. What does it mean?
When flag `new-lock` = `true` go-composer trying create go-composer.lock, but if
have composer.lock, go-composer will choose composer.lock and go-composer.lock don't will create.

Flag `force-new-lock` using by default=false. What does if mean?
When flag `force-new-lock` = `true` go-composer create go-composer.lock
regardless of whether the file exists(composer.lock) or not

> By default go-composer using go-composer.lock

## 🔧 Advanced Usage

### Flags

```bash
# Verbose output
go-composer install -v

# Skip dev dependencies
go-composer install --no-dev

# Custom working directory
go-composer install -d /path/to/project

# Skip autoloader generation
go-composer install --no-autoloader
```

### Example: Requiring Multiple Packages

```bash
go-composer require \
  symfony/console:^5.0 \
  monolog/monolog:^2.0 \
  guzzlehttp/guzzle:^7.0
```

## 🐛 Known Limitations

- ⚠️ Composer scripts are not executed
- ⚠️ Composer plugins are not supported
- ⚠️ Platform requirements (`php`, `ext-*`) are detected but not validated
- ⚠️ Some Symfony projects may need cache clearing: `rm -rf var/cache/*`

## 🛠️ Development

### Prerequisites
- Go 1.21 or higher
- Make (optional, for using Makefile)

### Building

```bash
# Development build
go build -o go-composer .

# Production build with optimizations
make build

# Run tests
go test ./...

# Install globally
sudo make install
```

### Testing

```bash
# Test on simple example
cd examples/simple-monolog
rm -rf vendor composer.lock
../go-composer install
php index.php
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License with Attribution Requirements - see the [LICENSE](LICENSE) file for details.

**Important**: Modified versions of this software must:
- Include a clear reference to the original source: https://github.com/xman12/go-composer
- Clearly indicate that changes have been made
- Not misrepresent the origin of the software

📋 See [ATTRIBUTION.md](ATTRIBUTION.md) for detailed attribution requirements and examples.

## Acknowledgments

- [Composer](https://getcomposer.org/) - The original and amazing PHP dependency manager
- [Packagist](https://packagist.org/) - The PHP package repository
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Semver](https://github.com/Masterminds/semver) - Semantic versioning library

## Success Stories

**go-composer** has been successfully tested with:
- ✅ Symfony 5.4 applications (36 packages)
- ✅ Laravel 10 applications (80+ packages)
- ✅ Monolog logging (3 packages)
- ✅ Guzzle HTTP client
- ✅ Doctrine ORM
- ✅ PHPUnit testing framework

---
If you find this project useful, please consider giving it a ⭐️ on GitHub!
