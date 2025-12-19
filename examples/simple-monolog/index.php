<?php

require_once __DIR__ . '/vendor/autoload.php';

use Monolog\Logger;
use Monolog\Handler\StreamHandler;

// Create a logger instance
$log = new Logger('my-app');

// Add a handler to write logs to stdout
$log->pushHandler(new StreamHandler('php://stdout', Logger::DEBUG));

// Log some messages
$log->debug('This is a debug message');
$log->info('Application started successfully');
$log->warning('This is a warning message');
$log->error('An error occurred');

echo "\n✅ Monolog example completed successfully!\n";

