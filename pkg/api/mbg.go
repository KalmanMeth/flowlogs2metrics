package api

import "time"

type MbgConnectionInfo struct {
	ServiceName      string
	ConnectionToken  string
	LocalSourceIP    string
	LocalSourcePort  string
	GatewayIP        string
	GatewayPort      string
	ExpireTime       time.Time
	ConnectionActive bool
}
