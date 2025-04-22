package client

import (
	"fmt"
	"net"
	"net/http"
	"runtime"
	"syscall"
	"time"
)

// Code below mostly from https://www.agwa.name/blog/post/preventing_server_side_request_forgery_in_golang

func safeSocketControl(network string, address string, conn syscall.RawConn) error {
	if !(network == "tcp4" || network == "tcp6") {
		return fmt.Errorf("%s is not a safe network type", network)
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%s is not a valid host/port pair: %s", address, err)
	}

	ipaddress := net.ParseIP(host)
	if ipaddress == nil {
		return fmt.Errorf("%s is not a valid IP address", host)
	}

	if !isPublicIPAddress(ipaddress) {
		return fmt.Errorf("%s is not a public IP address", ipaddress)
	}

	if !(port == "80" || port == "443") {
		return fmt.Errorf("%s is not a safe port number", port)
	}

	return nil
}

// Some of the below conf values from https://github.com/imgproxy/imgproxy/blob/master/transport/transport.go

const ClientKeepAliveTimeout = 90       // Imgproxy default
var Workers = runtime.GOMAXPROCS(0) * 2 // Imgproxy default

func NewHTTPClient(auth string) (*http.Client, error) {
	safeDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
		Control:   safeSocketControl,
	}

	safeTransport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           safeDialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   Workers + 1,
		IdleConnTimeout:       time.Duration(ClientKeepAliveTimeout) * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: newAuthenticatedRoundTripper(auth, safeTransport),
	}, nil
}
