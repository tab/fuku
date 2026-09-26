package main

import "fuku/e2e/services"

func main() {
	services.NewHTTP("http-api", "127.0.0.1:19881").Run()
}
