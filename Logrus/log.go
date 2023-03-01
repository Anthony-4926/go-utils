package log

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
)

const (
	allLog   = "all"
	errLog   = "err"
	warnLog  = "warn"
	infoLog  = "info"
	fatalLog = "fatal"
)

type FileLevelHook struct {
	file      *os.File
	errFile   *os.File
	warnFile  *os.File
	infoFile  *os.File
	fatalFile *os.File
	logPath   string
}

func (hook FileLevelHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook FileLevelHook) Fire(entry *logrus.Entry) error {
	line, _ := entry.String()
	switch entry.Level {
	case logrus.ErrorLevel:
		hook.errFile.Write([]byte(line))
	case logrus.WarnLevel:
		hook.warnFile.Write([]byte(line))
	case logrus.InfoLevel:
		hook.infoFile.Write([]byte(line))
	case logrus.FatalLevel:
		hook.fatalFile.Write([]byte(line))
	}
	hook.file.Write([]byte(line))
	return nil
}

func InitLog(logPath string) {
	logrus.SetLevel(logrus.InfoLevel)

	initLevel(logPath)
	logrus.SetFormatter(&logrus.TextFormatter{TimestampFormat: "2006/01/02 15:04:05"})
}

func initLevel(logPath string) {
	err := os.MkdirAll(fmt.Sprintf("%s", logPath), os.ModePerm)
	if err != nil {
		logrus.Error(err)
		return
	}
	errFile, err := os.OpenFile(fmt.Sprintf("%s/%s.log", logPath, errLog), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	warnFile, err := os.OpenFile(fmt.Sprintf("%s/%s.log", logPath, warnLog), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	infoFile, err := os.OpenFile(fmt.Sprintf("%s/%s.log", logPath, infoLog), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	fatalFile, err := os.OpenFile(fmt.Sprintf("%s/%s.log", logPath, fatalLog), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	fileHook := FileLevelHook{
		errFile:   errFile,
		warnFile:  warnFile,
		infoFile:  infoFile,
		fatalFile: fatalFile,
		logPath:   logPath}
	logrus.AddHook(&fileHook)
}
