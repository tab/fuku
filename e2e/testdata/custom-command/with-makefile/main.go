package main

import "fuku/e2e/services"

func main() {
	services.NewLog("with-makefile").Run()
}
