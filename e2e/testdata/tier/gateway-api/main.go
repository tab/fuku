package main

import "fuku/e2e/services"

func main() {
	services.NewHTTP("gateway-api", ":18080").Run()
}
