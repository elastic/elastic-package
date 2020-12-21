// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"syscall"
	"time"
)

var (
	dest  string
	proto string
	delay time.Duration
)

func init() {
	flag.StringVar(&dest, "dest", "localhost:514", "destination tcp address")
	flag.StringVar(&proto, "proto", "tcp", "protocol to use (tcp or udp)")
	flag.DurationVar(&delay, "delay", 0, "delay between messages")
}

func main() {
	log.SetFlags(0)
	flag.Parse()

	var err error
	fmt.Fprintf(os.Stderr, "Using proto=%s dest=%s\n", proto, dest)
	var conn net.Conn
	for {
		conn, err = net.Dial(proto, dest)
		if err != nil {
			log.Println(err)
			time.Sleep(time.Second)
			continue
		}
		break
	}
	defer conn.Close()

	for _, input := range flag.Args() {
		fmt.Fprintf(os.Stderr, "Delivering file %v\n", input)
		f, err := os.Open(input)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		count := 0

		for scanner.Scan() {
			count += 1
			time.Sleep(delay)
			data := append(scanner.Bytes(), '\n')
			n, err := conn.Write(data)
			if err != nil || n != len(data) {
				if proto == "udp" && errors.Is(err, syscall.ECONNREFUSED) {
					time.Sleep(time.Second)
					log.Printf("Restarted count=%d", count)
					f.Seek(0, 0)
					scanner = bufio.NewScanner(f)
					count = 0
					continue
				}
				log.Fatalf("Error sending message %d: %v", count, err)
			}
		}
	}
}
