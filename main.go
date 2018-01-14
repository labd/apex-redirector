package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	vhost "github.com/inconshreveable/go-vhost"
)

type Options struct {
	bindHTTP  string
	bindHTTPS string
	secret    string
}

func parseArgs() *Options {
	var bindHTTP, bindHTTPS, secret string
	secretEnv := os.Getenv("APEXREDIRECTOR_SECRET")

	flag.StringVar(&bindHTTP, "bindHTTP", "127.0.0.1:8080", "The HTTP address to listen on")
	flag.StringVar(&bindHTTPS, "bindHTTPS", "127.0.0.1:8443", "The HTTPS address to listen on")
	flag.StringVar(&secret, "secret", "", "The secret token to validate proxy requests")
	flag.Parse()

	if secret == "" {
		secret = secretEnv
		if secret == "" {
			log.Fatal("Please supply a secret (--secret)")
		}
	}

	return &Options{
		bindHTTP:  bindHTTP,
		bindHTTPS: bindHTTPS,
		secret:    secret,
	}
}

func startHTTPProxy(opts *Options) error {
	listener, err := net.Listen("tcp", opts.bindHTTP)
	if err != nil {
		log.Fatalf("Unable to bind HTTP on %s (\"%s\")", opts.bindHTTP, err)
		return err
	}
	defer listener.Close()

	log.Printf("HTTP proxy listing on %s", opts.bindHTTP)

	for {

		// accept a new connection
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go func(conn net.Conn) {
			defer conn.Close()

			var vhostConn *vhost.HTTPConn

			// parse out the HTTP request and the Host header
			if vhostConn, err = vhost.HTTP(conn); err != nil {
				log.Print("Invalid HTTP connection")
				return
			}
			defer vhostConn.Close()
			proxyConnection(opts, vhostConn, vhostConn.Host(), 80)
		}(conn)

	}
}

func startHTTPSProxy(opts *Options) error {
	listener, err := net.Listen("tcp", opts.bindHTTPS)
	if err != nil {
		log.Fatalf("Unable to bind HTTP on %s (\"%s\")", opts.bindHTTPS, err)
		return err
	}
	defer listener.Close()

	log.Printf("HTTPS proxy listing on %s", opts.bindHTTPS)

	for {
		// accept a new connection
		conn, _ := listener.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			var vhostConn *vhost.TLSConn

			// parse out the HTTP request and the Host header
			if vhostConn, err = vhost.TLS(conn); err != nil {
				log.Print("Invalid TLS connection")
				return
			}
			defer vhostConn.Close()

			proxyConnection(opts, vhostConn, vhostConn.Host(), 443)
		}(conn)
	}
}

func getTargetHost(opts *Options, address string, defaultPort int) (string, error) {
	var host string
	var port string

	if strings.Contains(address, ":") {
		var err error
		host, port, err = net.SplitHostPort(address)
		if err != nil {
			log.Println(err)
			return "", err
		}
	} else {
		host = address
		port = strconv.Itoa(defaultPort)
	}

	var lookupHostname string
	var addresses []string
	var err error

	// Validate that we are allowed to proxy to the host. This is done by
	// comparing a HMAC key on the TXT record
	redirectKey := createHmac256(host, opts.secret)
	lookupHostname = fmt.Sprintf("_apexredirector.%s", host)
	addresses, err = net.LookupTXT(lookupHostname)
	if err != nil || addresses[0] != redirectKey {
		err := errors.New("No matching TXT record")
		log.Printf(
			"Error: Proxy request not allowed - expected TXT record _apexredirector.%s with value %s",
			host, redirectKey)
		return "", err
	}

	lookupHostname = fmt.Sprintf("www.%s", host)
	addresses, err = net.LookupHost(lookupHostname)

	if err != nil {
		log.Println(err)
		return "", err
	}

	dest := fmt.Sprintf("%s:%s", addresses[0], port)
	return dest, nil
}

func proxyConnection(opts *Options, srcConn net.Conn, srcAddr string, dstPort int) error {

	dstAddr, err := getTargetHost(opts, srcAddr, dstPort)
	if err != nil {
		log.Printf("No proxy target found for %s", srcAddr)
		return err
	}

	log.Printf("Proxying request from %s to %s", srcAddr, dstAddr)

	dstConn, err := net.Dial("tcp", dstAddr)
	if err != nil {
		log.Printf("Unable to open connection to backend %s: %v\n", dstAddr, err)
		return err
	}

	errc := make(chan error, 2)
	cp := func(dst io.Writer, src io.Reader) {
		_, err := io.Copy(dst, src)
		errc <- err
	}
	go cp(dstConn, srcConn)
	go cp(srcConn, dstConn)
	<-errc

	return nil
}

func createHmac256(message string, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func main() {
	opts := parseArgs()

	log.Print("Starting apexredirector..")
	go startHTTPProxy(opts)
	startHTTPSProxy(opts)
}
