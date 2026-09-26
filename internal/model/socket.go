package model

// Socket is one instance socket file: its path, whether it exists, whether an instance answers and its dial error
type Socket struct {
	Path      string
	Present   bool
	Reachable bool
	Error     error
}

// SocketScan lists the socket files found in the socket directory (Files counts every match, Sockets the real sockets)
type SocketScan struct {
	Dir     string
	Pattern string
	Files   int
	Sockets []Socket
}
