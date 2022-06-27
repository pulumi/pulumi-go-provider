package main

type Connection struct {
	Type
	port     int
	username string
}

func main() {
	serializeResource("pkgname", "resourcename", Connection{})
}
