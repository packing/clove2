package base

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func GeneratePID(pidFile string) {
	pf, err := os.OpenFile(pidFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0664)
	if err != nil {
		LogError("Failed to write pid to file!", err)
		return
	}

	pf.WriteString(fmt.Sprintf("%d\n", os.Getpid()))
	pf.Close()
}

func RemovePID(pidFile string) {
	err, pids := ReadPIDs(pidFile)
	if err != nil {
		LogError("Failed to delete PID!", err)
		return
	}

	pf, err := os.OpenFile(pidFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0664)
	if err != nil {
		LogError("Failed to delete PID!", err)
		return
	}

	for _, pid := range pids {
		if pid == os.Getpid() {
			continue
		}
		pf.WriteString(fmt.Sprintf("%d\n", pid))
	}
	pf.Close()
}

func ReadPIDs(pidFile string) (error, []int) {
	pf, err := os.Open(pidFile)
	if err != nil {
		LogError("The PID file does not exist or cannot be opened!", err)
		return err, nil
	}

	bs := make([]byte, 10240)
	n, err := pf.Read(bs)
	pf.Close()
	if err != nil {
		LogError("Failed to read pid from file!", err)
		return err, nil
	}
	ctx := string(bs[:n])
	pids := strings.Split(ctx, "\n")
	intpids := make([]int, len(pids))
	i := 0
	for _, v := range pids {
		nv, err := strconv.Atoi(v)
		if err == nil {
			intpids[i] = nv
			i++
		}
	}
	return nil, intpids[:i]
}
