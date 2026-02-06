package main

import "fuku/e2e/services"

func main() {
	services.NewTCP("postgres", ":5432").Run()
}
