package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Logger struct {
	LogDir     string
	SystemLog  string
	NetworkLog string
	Cond       string
	Print      bool
	NodeId     int
	NetOnly    bool
}

var (
	LogCond = map[string]int{
		"emergency":   0,
		"alert":       1,
		"critical":    2,
		"error":       3,
		"warning":     4,
		"notice":      5,
		"information": 6,
		"debug":       7,
	}
	sysLogMutex = &sync.Mutex{}
	netLogMutex = &sync.Mutex{}
)

func InitLogger(logDir string, systemLog string, networkLog string, cond string, print bool, nodeId int, notsys bool) (Logger, error) {
	logger := Logger{}
	logger.Cond = cond
	logger.Print = print
	logger.NodeId = nodeId
	logger.NetOnly = notsys
	err := logger.SetLogDir(logDir)
	if err != nil {
		return Logger{}, err
	}

	err = logger.initSystemLog(systemLog)
	if err != nil {
		return Logger{}, err
	}

	err = logger.initNetworkLog(networkLog)
	if err != nil {
		return Logger{}, err
	}

	if _, ok := LogCond[cond]; ok == false {
		return Logger{}, errors.New("Logger: Unknown cond level \"" + cond + "\" given")
	}

	logger.LogDir = logDir

	return logger, nil
}

func (log *Logger) SetLogDir(name string) error {
	if _, err := os.Stat(name); os.IsNotExist(err) {
		err := os.Mkdir(name, os.ModePerm)
		if err != nil {
			return errors.New("Logger: Could not create log directory with path: " + name)
		}
	}
	log.LogDir = name
	return nil
}

func (log *Logger) initSystemLog(name string) error {
	nodeName := name + "_" + strconv.Itoa(log.NodeId) + ".txt"
	fileName := filepath.Join(log.LogDir, nodeName)
	fmt.Println(fileName)
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	file.Close()
	if err != nil {
		return errors.New("Logger: Failed to set system log file to: " + fileName)
	}
	log.SystemLog = nodeName
	return nil
}

func (log *Logger) initNetworkLog(name string) error {
	nodeName := name + "_" + strconv.Itoa(log.NodeId) + ".txt"
	fileName := filepath.Join(log.LogDir, nodeName)
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	file.Close()
	if err != nil {
		return errors.New("Logger: Failed to set system log file to: " + fileName)
	}
	log.NetworkLog = nodeName
	return nil
}

func (log *Logger) Log(dest string, cond string, msg string) error {
	if _, ok := LogCond[cond]; ok == false {
		return errors.New("Logger: Unknown cond level \"" + cond + "\" given")
	}
	if LogCond[cond] > LogCond[log.Cond] {
		return nil
	}

	if dest == "system" {
		return log.logSystem(cond, msg)
	} else if dest == "network" {
		return log.logNetwork(cond, msg)
	}
	return nil
}

func (log *Logger) logSystem(cond string, msg string) error {
	//Create message:
	timeStamp := time.Now().Format(time.RFC3339)
	logMessage := timeStamp + "  -  " + strings.ToUpper(cond) + "  -  " + msg
	if log.Print == true && log.NetOnly == false {
		fmt.Println(logMessage)
	}

	fileName := filepath.Join(log.LogDir, log.SystemLog)
	sysLogMutex.Lock()
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.New("Logger: Failed to open system log file, with error: " + err.Error())
	}

	_, err = file.WriteString(logMessage + "\n")
	file.Close()
	sysLogMutex.Unlock()
	if err != nil {
		return errors.New("Logger: Failed to write to system log file, with error: " + err.Error())
	}
	return nil
}

func (log *Logger) logNetwork(cond string, msg string) error {
	//Create message:
	timeStamp := time.Now().Format(time.RFC3339)
	logMessage := timeStamp + "  -  " + strings.ToUpper(cond) + "  -  " + msg
	if log.Print == true {
		fmt.Println(logMessage)
	}

	fileName := filepath.Join(log.LogDir, log.NetworkLog)
	netLogMutex.Lock()
	file, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.New("Logger: Failed to open network log file, with error: " + err.Error())
	}
	_, err = file.WriteString(logMessage + "\n")
	file.Close()
	netLogMutex.Unlock()
	if err != nil {
		return errors.New("Logger: Failed to write to network log file, with error: " + err.Error())
	}
	return nil
}