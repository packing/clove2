// +build windows

package base

import "fmt"

func Daemon() {
	fmt.Printf("Daemons is not supported on Windows.")
}
