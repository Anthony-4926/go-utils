package goRoutine

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"runtime"
	"strconv"
	"strings"
)

func GoId() int {
	defer func() {
		if err := recover(); err != nil {
			logrus.Errorf("panic recover: %v", err)
		}
	}()

	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idFiled := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idFiled)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return id
}
