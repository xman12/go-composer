# go-composer Examples / Примеры использования

Эта папка содержит примеры использования go-composer с различными PHP библиотеками и фреймворками.

This folder contains examples of using go-composer with various PHP libraries and frameworks.

## 📁 Available Examples / Доступные примеры

### 1. [simple-monolog](./simple-monolog/)

**Complexity**: Beginner / Начальный  
**Packages**: 3 (monolog/monolog + dependencies)  
**Time**: ~2 seconds

Минимальный пример использования популярной библиотеки логирования Monolog.

A minimal example using the popular Monolog logging library.

**Features**:
- ✅ Basic dependency installation
- ✅ PSR-4 autoloading
- ✅ Simple logging example
- ✅ Fast parallel downloads

**Quick start**:
```bash
cd simple-monolog
../../go-composer install
php index.php
```

---

## 🚀 How to Use Examples / Как использовать примеры

### For each example / Для каждого примера:

1. **Navigate to the example directory** / **Перейдите в папку примера**:
   ```bash
   cd examples/simple-monolog
   ```

2. **Install dependencies** / **Установите зависимости**:
   ```bash
   ../../go-composer install
   ```

3. **Run the example** / **Запустите пример**:
   ```bash
   php index.php
   ```

4. **Clean up (optional)** / **Очистка (опционально)**:
   ```bash
   rm -rf vendor go-composer.lock
   ```

## 📊 Performance Comparison / Сравнение производительности

All examples can be tested with both go-composer and traditional Composer:

Все примеры можно протестировать как с go-composer, так и с обычным Composer:

```bash
# Test with go-composer
time ../../go-composer install

# Clean up
rm -rf vendor go-composer.lock

# Test with PHP Composer
time composer install
```

Expected speedup: **3-5x faster** ⚡️

Ожидаемое ускорение: **в 3-5 раз быстрее** ⚡️

## 🎯 What These Examples Demonstrate / Что демонстрируют эти примеры

✅ **Installing packages from Packagist** / **Установка пакетов из Packagist**  
✅ **Automatic dependency resolution** / **Автоматическое разрешение зависимостей**  
✅ **PSR-4 autoloader generation** / **Генерация PSR-4 автозагрузчика**  
✅ **Lock file for reproducible builds** / **Lock-файл для воспроизводимых сборок**  
✅ **Parallel downloads** / **Параллельная загрузка**  
✅ **Composer 2 compatibility** / **Совместимость с Composer 2**  

## 🔧 Troubleshooting / Решение проблем

### Error: "composer.json not found"
Make sure you're in the example directory with a `composer.json` file.

Убедитесь, что вы находитесь в папке примера с файлом `composer.json`.

### Error: "failed to fetch package"
Check your internet connection and Packagist availability.

Проверьте интернет-соединение и доступность Packagist.

### Autoload errors
Make sure to run `../../go-composer install` before running PHP scripts.

Убедитесь, что запустили `../../go-composer install` перед запуском PHP скриптов.

## 📝 Adding Your Own Examples / Добавление своих примеров

Feel free to add your own examples! Each example should contain:

Не стесняйтесь добавлять свои примеры! Каждый пример должен содержать:

1. `composer.json` - project configuration
2. `README.md` - detailed instructions  
3. `index.php` or similar - demonstration script
4. `.gitignore` - to exclude vendor/ and lock files

## 📚 More Information / Дополнительная информация

- [Main README](../README.md) - Project overview
- [Quick Start Guide](../QUICKSTART.md) - Get started in 5 minutes
- [Attribution Requirements](../ATTRIBUTION.md) - License information

## 🤝 Contributing / Вклад в проект

If you create a useful example, please consider submitting a Pull Request!

Если вы создали полезный пример, пожалуйста, отправьте Pull Request!

---

**Original source**: https://github.com/xman12/go-composer  
**Copyright**: (c) 2025 Aleksandr Belyshev

