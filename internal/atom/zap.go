package atom

import (
	"errors"
	"log"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var levelMap = map[string]zapcore.Level{
	"debug": zapcore.DebugLevel,
	"info":  zapcore.InfoLevel,
	"warn":  zapcore.WarnLevel,
	"error": zapcore.ErrorLevel,
}

var Log *zap.SugaredLogger
var zaplogger *zap.Logger

func GetZapLogger() *zap.Logger {
	return zaplogger
}

func InitZap(level string, dir string) error {
	zapLevel, ok := levelMap[level]
	if !ok {
		log.Fatalf("illegal log level: %s", level)
		return errors.New("illegal log level")
	}
	writeSyncer := getWriteSyncer(dir)
	encoder := getEncoder()
	core := zapcore.NewCore(encoder, writeSyncer, zapLevel)

	zaplogger = zap.New(core, zap.AddCaller())
	Log = zaplogger.Sugar()

	Log.Infow("sugar log test1",
		"url", "http://example.com",
		"attempt", 3,
		"backoff", time.Second,
	)

	Log.Infof("sugar log test2: %s", "http://example.com")

	return nil
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.FunctionKey = "func"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.ConsoleSeparator = "|"
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func getWriteSyncer(dir string) zapcore.WriteSyncer {
	return zapcore.AddSync(&lumberjack.Logger{
		Filename:   "log.log",
		MaxSize:    500, // megabytes
		MaxBackups: 3,
		MaxAge:     28,    //days
		Compress:   false, // disabled by default
	})

	// hook, err := rotatelogs.New(
	// 	dir+"/log"+".%Y%m%d",
	// 	rotatelogs.WithLinkName(dir+"/log"),
	// 	rotatelogs.WithMaxAge(time.Hour*24*7),
	// 	rotatelogs.WithRotationTime(time.Hour*24),
	// )

	// if err != nil {
	// 	log.Printf("failed to create rotatelogs: %s", err)
	// }
	// return zapcore.AddSync(hook)
}

/*
func InitZap(level string, dir string) error {
	logFile := "log.log"
	zapLevel, ok := levelMap[level]
	if !ok {
		log.Fatalf("illegal log level: %s", level)
		return errors.New("illegal log level")
	}
	zap.RegisterSink("lumberjack", func(*url.URL) (zap.Sink, error) {
		return lumberjackSink{
			Logger: getLumberjackLogger(dir),
		}, nil
	})
	loggerConfig := zap.Config{
		Level:         zap.NewAtomicLevelAt(zapLevel),
		Development:   true,
		Encoding:      "console",
		EncoderConfig: getEncoderConfig(),
		OutputPaths:   []string{fmt.Sprintf("lumberjack:%s", logFile)},
	}
	zaplogger, err := loggerConfig.Build()
	if err != nil {
		panic(fmt.Sprintf("build zap logger from config error: %v", err))
	}
	zap.ReplaceGlobals(zaplogger)
	Log = zaplogger.Sugar() // NewSugar("sugar")

	sugar.Infow("sugar log test1",
		"url", "http://example.com",
		"attempt", 3,
		"backoff", time.Second,
	)

	sugar.Infof("sugar log test2: %s", "http://example.com")

	return nil
}

func NewSugar(name string) *zap.SugaredLogger {
	return zaplogger.Named(name).Sugar()
}

func getEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:          "ts",
		LevelKey:         "level",
		NameKey:          "logger",
		CallerKey:        "caller",
		MessageKey:       "msg",
		StacktraceKey:    "stacktrace",
		FunctionKey:      "func",
		ConsoleSeparator: "|",
		LineEnding:       zapcore.DefaultLineEnding,
		EncodeLevel:      zapcore.CapitalLevelEncoder,
		EncodeTime:       zapcore.ISO8601TimeEncoder,
		EncodeDuration:   zapcore.SecondsDurationEncoder,
		EncodeCaller:     zapcore.ShortCallerEncoder,
	}
}

type lumberjackSink struct {
	*lumberjack.Logger
}

func (lumberjackSink) Sync() error {
	return nil
}

func getLumberjackLogger(dir string) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   "log.log",
		MaxSize:    500, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   //days
		Compress:   true, // disabled by default
	}
}
*/
