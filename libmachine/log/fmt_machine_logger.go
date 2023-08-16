package log

import (
	"fmt"
	"io"
	"log"
	"os"
)

type FmtMachineLogger struct {
	outWriter io.Writer
	errWriter io.Writer
	debug     bool
	history   *HistoryRecorder
	logFile   *os.File
}

func openLogFile() *os.File {
	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		log.Fatalf("Could not get user home directory: %s", homeErr.Error())
	}

	machineDir := fmt.Sprintf("%s/.docker/machine", homeDir)
	_, statErr := os.Stat(machineDir)
	if os.IsNotExist(statErr) {
		createErr := os.MkdirAll(machineDir, 0700)
		if createErr != nil {
			log.Fatalf("Could not create machine directory: %s", createErr.Error())
		}
	}

	file, err := os.OpenFile(fmt.Sprintf("%s/docker-machine.log", machineDir), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		log.Fatalf("Could not open log file: %s", err.Error())
	}

	return file
}

// NewFmtMachineLogger creates a MachineLogger implementation used by the drivers
func NewFmtMachineLogger() MachineLogger {
	return &FmtMachineLogger{
		outWriter: os.Stdout,
		errWriter: os.Stderr,
		debug:     false,
		history:   NewHistoryRecorder(),
		logFile:   openLogFile(),
	}
}

func (ml *FmtMachineLogger) SetDebug(debug bool) {
	ml.debug = debug
}

func (ml *FmtMachineLogger) SetOutWriter(out io.Writer) {
	ml.outWriter = out
}

func (ml *FmtMachineLogger) SetErrWriter(err io.Writer) {
	ml.errWriter = err
}

func (ml *FmtMachineLogger) writeErr(str string) {
	ml.logFile.WriteString(str)
	fmt.Fprint(ml.errWriter, str)
}

func (ml *FmtMachineLogger) writeOut(str string) {
	ml.logFile.WriteString(str)
	fmt.Fprint(ml.outWriter, str)
}

func (ml *FmtMachineLogger) Debug(args ...interface{}) {
	ml.history.Record(args...)
	if ml.debug {
		str := fmt.Sprintln(args...)
		ml.writeErr(str)
	}
}

func (ml *FmtMachineLogger) Debugf(fmtString string, args ...interface{}) {
	ml.history.Recordf(fmtString, args...)
	if ml.debug {
		str := fmt.Sprintf(fmtString+"\n", args...)
		ml.writeErr(str)
	}
}

func (ml *FmtMachineLogger) Error(args ...interface{}) {
	ml.history.Record(args...)
	str := fmt.Sprintln(args...)
	ml.writeErr(str)
}

func (ml *FmtMachineLogger) Errorf(fmtString string, args ...interface{}) {
	ml.history.Recordf(fmtString, args...)
	str := fmt.Sprintf(fmtString+"\n", args...)
	ml.writeErr(str)
}

func (ml *FmtMachineLogger) Info(args ...interface{}) {
	ml.history.Record(args...)
	str := fmt.Sprintln(args...)
	ml.writeOut(str)
}

func (ml *FmtMachineLogger) Infof(fmtString string, args ...interface{}) {
	ml.history.Recordf(fmtString, args...)
	str := fmt.Sprintf(fmtString+"\n", args...)
	ml.writeOut(str)
}

func (ml *FmtMachineLogger) Warn(args ...interface{}) {
	ml.history.Record(args...)
	str := fmt.Sprintln(args...)
	ml.writeOut(str)
}

func (ml *FmtMachineLogger) Warnf(fmtString string, args ...interface{}) {
	ml.history.Recordf(fmtString, args...)
	str := fmt.Sprintf(fmtString+"\n", args...)
	ml.writeOut(str)
}

func (ml *FmtMachineLogger) History() []string {
	return ml.history.records
}
