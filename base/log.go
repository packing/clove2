package base

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"time"
)

const (
	LogLevelVerbose = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

var LogLevelTags = []string{"V", "I", "W", "E"}

var logLevel = LogLevelVerbose
var logPrefix string

func LogInit(level int, prefix string) error {
	logLevel = level
	logPrefix = prefix
	if len(logPrefix) > 0 {
		dt := time.Now().Format("2006-01-02")
		fn := fmt.Sprintf("%s-%s.log", logPrefix, dt)
		_, err := os.Stat(fn)
		if os.IsExist(err) {
			for j := 1; j < 100; j++ {
				sfn := fmt.Sprintf("%s.%d", fn, j)
				_, err := os.Stat(sfn)
				if os.IsExist(err) {
					continue
				}
				_ = os.Rename(fn, sfn)
			}
		}
		pf, err := os.Create(fn)
		if err == nil {
			_ = pf.Close()
		} else {
			return err
		}
	}

	return nil
}

func getLogIO() (error, io.Writer) {
	dt := time.Now().Format("2006-01-02")
	fn := fmt.Sprintf("%s-%s.log", logPrefix, dt)
	pf, err := os.OpenFile(fn, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err, nil
	}
	return nil, pf
}

func logAndLevel(level int, format string, v ...interface{}) {
	if level < logLevel {
		return
	}
	s := fmt.Sprintf(format, v...)
	var logger *log.Logger = nil
	if len(logPrefix) > 0 {
		err, iowriter := getLogIO()
		if err != nil {
			return
		}
		logger = log.New(iowriter, fmt.Sprintf("[%s]", LogLevelTags[level]), log.Lmicroseconds)
		logger.Println(s)
	} else {
		log.SetFlags(log.Lmicroseconds)
		log.SetPrefix(fmt.Sprintf("[%s]", LogLevelTags[level]))
		log.Println(s)
	}
}

func LogRaw(s string) {
	var logger *log.Logger = nil
	if len(logPrefix) > 0 {
		err, iowriter := getLogIO()
		if err != nil {
			return
		}
		logger = log.New(iowriter, "", 0)
		logger.Println(s)
	} else {
		log.Println(s)
	}
}

func LogVerbose(format string, v ...interface{}) {
	logAndLevel(LogLevelVerbose, format, v...)
}

func LogInfo(format string, v ...interface{}) {
	logAndLevel(LogLevelInfo, format, v...)
}

func LogWarn(format string, v ...interface{}) {
	logAndLevel(LogLevelWarn, format, v...)
}

func LogError(format string, v ...interface{}) {
	logAndLevel(LogLevelError, format, v...)
}

func LogPanic(ierr interface{}) {
	if ierr == nil {
		return
	}
	err, ok := ierr.(error)
	if ok && err != nil {
		var st = func(all bool) string {
			buf := make([]byte, 512)
			for {
				size := runtime.Stack(buf, all)
				if size == len(buf) {
					buf = make([]byte, len(buf)*2)
					continue
				}
				break
			}

			return string(buf)
		}

		LogError("[!!!PANIC]", err, "\ncall tack:"+st(false))
	}
}
