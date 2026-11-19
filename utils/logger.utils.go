package utils

import (
	"log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"os"
)

// Log is our global instance of the Logger.
// We use SugaredLogger because it allows easy formating (e.g., Log.Infof("User %d",id))
//
// It starts as a NO-OP logger instead of nil. main() replaces it with the real
// one via InitLogger, but any code path that logs before that (or in a test that
// never installs a logger) must not be able to panic a request with a nil
// pointer: logging is observability, never a failure mode. The `if Log != nil`
// guards elsewhere stay harmless.
var Log *zap.SugaredLogger = zap.NewNop().Sugar()

// LoggerConfig defines the configuraton structure we discussed
type LoggerConfig struct {
	Level      string // "debug", "info", "warn", "error"
	Format     string // "json", "console"
	OutputFile string // Optional: path to a text file
}

// InitLogger initializes the logging system according to the provided configuration.
func InitLogger(cfg LoggerConfig) {
	var writer zapcore.WriteSyncer

	// 1. We define the destination (Output)
	if cfg.OutputFile != "" {
		file, err := os.OpenFile(cfg.OutputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			// 1. We use the standard Go log to report the emergency in the console
			log.Printf("⚠️EMERGENCY ALERT: COuld not open log file '%s':%v. Usingdefault console.", cfg.OutputFile, err)
			// 2. We perform a safe fallback to the standard console (os.Stdout)
			writer = zapcore.AddSync(os.Stdout)
		} else {
			// if everything went well, we use the file
			writer = zapcore.AddSync(file)
		}
	} else {
		// By default, we write to the console (standard output)
		writer = zapcore.AddSync(os.Stdout)
	}
	// 2. We define the Log Level (Level)
	var level zapcore.Level
	switch cfg.Level {
	case "debug":
		level = zap.DebugLevel
	case "warn":
		level = zap.WarnLevel
	case "error":
		level = zap.ErrorLevel
	case "panic":
		level = zap.PanicLevel
	case "fatal":
		level = zap.FatalLevel
	default:
		level = zap.InfoLevel
	}
	// 3. Config the fields and message format
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder        // Readable date format (YYYY-MM-DD-HH:MM:SS)
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder // Colors for the console

	// 4. Choose the encoder according to the format
	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		// In Prodution, we don't want colors in the JSON, so we use the standard encoder
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		// Friendly format for the local console
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 5. Assemble the zap (core)
	core := zapcore.NewCore(encoder, writer, level)

	// 6. Create the Logger by adding the "Caller" (File and line of code)
	logger := zap.New(core, zap.AddCaller())

	// Set the global variable
	Log = logger.Sugar()
}
