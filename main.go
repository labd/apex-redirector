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

type options struct {
	bindHTTP  string
	bindHTTPS string
	secret    string
}

type Server struct {
	options *options
}

func parseArgs() *options {
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

	return &options{
		bindHTTP:  bindHTTP,
		bindHTTPS: bindHTTPS,
		secret:    secret,
	}
}

func (s Server) startHTTPProxy() error {
	listener, err := net.Listen("tcp", s.options.bindHTTP)
	if err != nil {
		log.Fatalf("Unable to bind HTTP on %s (\"%s\")", s.options.bindHTTP, err)
		return err
	}
	defer listener.Close()

	log.Printf("HTTP proxy listing on %s", s.options.bindHTTP)

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
			vhostConn, err = vhost.HTTP(conn)

			defer vhostConn.Free()
			defer vhostConn.Close()
			if err != nil {
				return
			}
			s.proxyConnection(vhostConn, vhostConn.Host(), 80)
		}(conn)

	}
}

func (s Server) startHTTPSProxy() error {
	listener, err := net.Listen("tcp", s.options.bindHTTPS)
	if err != nil {
		log.Fatalf("Unable to bind HTTP on %s (\"%s\")", s.options.bindHTTPS, err)
		return err
	}
	defer listener.Close()

	log.Printf("HTTPS proxy listing on %s", s.options.bindHTTPS)

	for {
		// accept a new connection
		conn, err := listener.Accept()

		if err != nil {
			conn.Close()
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			var vhostConn *vhost.TLSConn

			// parse out the HTTP request and the Host header
			vhostConn, err = vhost.TLS(conn)

			defer vhostConn.Free()
			defer vhostConn.Close()
			if err != nil {
				return
			}
			s.proxyConnection(vhostConn, vhostConn.Host(), 443)
		}(conn)

	}
}

func (s Server) getTargetHost(address string, defaultPort int) (string, error) {
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
	redirectKey := createHmac256(host, s.options.secret)
	lookupHostname = fmt.Sprintf("_apex-redirector.%s", host)
	addresses, err = net.LookupTXT(lookupHostname)
	if err != nil || addresses[0] != redirectKey {
		err := errors.New("No matching TXT record")
		log.Printf(
			"Error: Proxy request not allowed - expected TXT record _apex-redirector.%s with value %s",
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

func (s Server) proxyConnection(srcConn net.Conn, srcAddr string, dstPort int) error {

	dstAddr, err := s.getTargetHost(srcAddr, dstPort)
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

func (s Server) start() {
	log.Print("Starting apex-redirector..")
	go s.startHTTPProxy()
	s.startHTTPSProxy()
}

func createHmac256(message string, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func main() {
	opts := parseArgs()

	server := Server{
		options: opts,
	}
	server.start()
}
