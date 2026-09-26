package main

import "fuku/e2e/services"

func main() {
	services.NewTCP("tcp-api", "127.0.0.1:19880").Run()
}
