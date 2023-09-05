package api

import "time"

type GatewayConnectionInfo struct {
	ServiceName      string
	ConnectionToken  string
	LocalSourceIP    string
	LocalSourcePort  string
	GatewayIP        string
	GatewayPort      string
	ExpireTime       time.Time
	ConnectionActive bool
}
