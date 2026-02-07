package main

import "fuku/e2e/services"

func main() {
	services.NewTCP("redis", ":6379").Run()
}
