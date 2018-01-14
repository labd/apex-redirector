package main

import (
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
	bindHTTPS string
	bindHTTP  string
}

func parseArgs() (*Options, error) {
	var bindHTTP string
	var bindHTTPS string

	flag.StringVar(&bindHTTP, "bindHTTP", "127.0.0.1:8080", "The HTTP address to listen on")
	flag.StringVar(&bindHTTPS, "bindHTTPS", "127.0.0.1:8443", "The HTTPS address to listen on")
	flag.Parse()

	return &Options{
		bindHTTP:  bindHTTP,
		bindHTTPS: bindHTTPS,
	}, nil
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
			proxyConnection(vhostConn, vhostConn.Host(), 80)
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

			proxyConnection(vhostConn, vhostConn.Host(), 443)
		}(conn)
	}
}

func getTargetHost(address string, defaultPort int) (string, error) {
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

	lookupHostname := fmt.Sprintf("www.%s", host)
	addresses, err := net.LookupHost(lookupHostname)
	if err != nil {
		log.Println(err)
		return "", err
	}

	dest := fmt.Sprintf("%s:%s", addresses[0], port)
	return dest, nil
}

func proxyConnection(srcConn net.Conn, srcAddr string, dstPort int) error {

	dstAddr, err := getTargetHost(srcAddr, dstPort)
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

func main() {
	opts, err := parseArgs()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	log.Print("Starting apexredirector..")
	go startHTTPProxy(opts)
	startHTTPSProxy(opts)
}
