package logger

import "fmt"

func Info(msg string) {
	fmt.Println("\033[34mℹ️ " + msg + "\033[0m")
}

func Success(msg string) {
	fmt.Println("\033[32m✅ " + msg + "\033[0m")
}

func Warn(msg string) {
	fmt.Println("\033[33m⚠️ " + msg + "\033[0m")
}

func Error(msg string) {
	fmt.Println("\033[31m❌ " + msg + "\033[0m")
}
