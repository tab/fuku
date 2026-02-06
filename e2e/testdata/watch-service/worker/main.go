package main

import "fuku/e2e/services"

func main() {
	services.NewLog("worker").Run()
}
