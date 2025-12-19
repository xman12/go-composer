# Simple Monolog Example

A minimal example demonstrating how to use **go-composer** with the popular Monolog logging library.

## 📋 What's included

- `composer.json` - Project configuration with Monolog dependency
- `index.php` - Simple logging example
- This demonstrates installing 3 packages: monolog/monolog, psr/log, and dependencies

## 🚀 Quick Start

### 1. Install dependencies using go-composer

```bash
# From the project root
cd examples/simple-monolog

# Install dependencies
../../go-composer install
```

You should see output like:
```
✅ Resolved 3 packages
⬇️  Downloading and installing packages...
Installing 100% |████████████████████| (3/3)
✅ All packages installed successfully!
🔧 Generating autoload files...
✅ Autoload files generated
🎉 Installation complete!
```

### 2. Run the example

```bash
php index.php
```

Expected output:
```
[2024-XX-XX HH:MM:SS] my-app.DEBUG: This is a debug message [] []
[2024-XX-XX HH:MM:SS] my-app.INFO: Application started successfully [] []
[2024-XX-XX HH:MM:SS] my-app.WARNING: This is a warning message [] []
[2024-XX-XX HH:MM:SS] my-app.ERROR: An error occurred [] []

✅ Monolog example completed successfully!
```

## 📦 Installed Packages

After running `go-composer install`, you'll have:

- `monolog/monolog` ^2.0 - The main logging library
- `psr/log` - PSR-3 logging interface
- Dependencies (automatically resolved)

## 🎯 What This Demonstrates

✅ Installing packages from Packagist
✅ Resolving dependencies automatically
✅ Generating PSR-4 autoloader
✅ Using go-composer.lock for reproducible builds
✅ Fast parallel downloads with Go

## 🧹 Cleanup

To start fresh:

```bash
rm -rf vendor go-composer.lock
../../go-composer install
```

## 🔄 Compare with PHP Composer

You can compare the speed with traditional Composer:

```bash
# Clean up
rm -rf vendor go-composer.lock

# Time go-composer
time ../../go-composer install

# Clean up again
rm -rf vendor go-composer.lock

# Time PHP composer (if installed)
time composer install
```

You should see **go-composer is 3-5x faster!** ⚡️

