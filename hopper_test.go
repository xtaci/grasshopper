// The MIT License (MIT)
//
// Copyright (c) 2024 xtaci
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package grasshopper

import (
	"crypto/sha1"
	"fmt"
	"log"
	"math/rand"
	"net"
	"testing"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

const SALT = "hopper test"

func newEchoServer(t *testing.T) *net.UDPConn {
	conn, err := net.ListenPacket("udp", "localhost:0")
	if err != nil {
		t.Fatalf("Error starting server: %v\n", err)
		return nil
	}

	t.Logf("UDP Echo Server is running on %v...", conn.LocalAddr())

	buffer := make([]byte, mtuLimit)

	go func() {
		for {
			n, clientAddr, err := conn.(*net.UDPConn).ReadFromUDP(buffer)
			if err != nil {
				t.Logf("Error reading data: %v\n", err)
				return
			}

			_, err = conn.(*net.UDPConn).WriteToUDP(buffer[:n], clientAddr)
			if err != nil {
				t.Logf("Error sending response: %v\n", err)
				return
			}
		}
	}()

	return conn.(*net.UDPConn)
}

func newHopper(listen string, nexthop []string, ki string, ko string, ci string, co string) *Listener {
	passIn := pbkdf2.Key([]byte(ki), []byte(SALT), 128, 32, sha1.New)
	passOut := pbkdf2.Key([]byte(ko), []byte(SALT), 128, 32, sha1.New)

	// init crypter
	crypterIn := newCrypt(passIn, ci)
	crypterOut := newCrypt(passOut, co)

	// init listener
	listener, err := ListenWithOptions(listen, nexthop, 1024*1024, 15*time.Second, crypterIn, crypterOut, nil, nil, log.Default())
	if err != nil {
		log.Fatal(err)
	}

	return listener
}

func TestHopperNone(t *testing.T) {
	conn := newEchoServer(t)

	ki, ko, ci, co := "", "", "none", "none"
	hop1 := newHopper("localhost:0", []string{conn.LocalAddr().String()}, ki, ko, ci, co)
	t.Log("Hop1:", hop1.conn.LocalAddr().String(), "->", conn.LocalAddr().String(), "ki:", ki, "ko:", ko, "ci:", ci, "co:", co)
	go hop1.Start()

	ki, ko, ci, co = "", "", "none", "none"
	hop2 := newHopper("localhost:0", []string{hop1.conn.LocalAddr().String()}, ki, ko, ci, co)
	t.Log("Hop2:", hop2.conn.LocalAddr().String(), "->", hop1.conn.LocalAddr().String(), "ki:", ki, "ko:", ko, "ci:", ci, "co:", co)
	go hop2.Start()

	clientConn, err := net.Dial("udp", hop2.conn.LocalAddr().String())
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer clientConn.Close()

	testEcho(t, clientConn)
}

func TestHopperAES(t *testing.T) {
	conn := newEchoServer(t)

	ki, ko, ci, co := "123456", "", "aes", "none"
	hop1 := newHopper("localhost:0", []string{conn.LocalAddr().String()}, ki, ko, ci, co)
	t.Log("Hop1:", hop1.conn.LocalAddr().String(), "->", conn.LocalAddr().String(), "ki:", ki, "ko:", ko, "ci:", ci, "co:", co)
	go hop1.Start()

	ki, ko, ci, co = "", "123456", "none", "aes"
	hop2 := newHopper("localhost:0", []string{hop1.conn.LocalAddr().String()}, ki, ko, ci, co)
	t.Log("Hop2:", hop2.conn.LocalAddr().String(), "->", hop1.conn.LocalAddr().String(), "ki:", ki, "ko:", ko, "ci:", ci, "co:", co)
	go hop2.Start()

	clientConn, err := net.Dial("udp", hop2.conn.LocalAddr().String())
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer clientConn.Close()

	testEcho(t, clientConn)
}

func TestMultiHoppers(t *testing.T) {
	var nextHops []string
	conn := newEchoServer(t)

	// create 10 LEVEL-2 hops
	ki, ko, ci, co := "123456", "", "aes", "none"
	for i := 0; i < 10; i++ {
		hop1 := newHopper("localhost:0", []string{conn.LocalAddr().String()}, ki, ko, ci, co)
		t.Log("Hop1:", hop1.conn.LocalAddr().String(), "->", conn.LocalAddr().String(), "ki:", ki, "ko:", ko, "ci:", ci, "co:", co)
		nextHops = append(nextHops, hop1.conn.LocalAddr().String())
		go hop1.Start()
	}
	fmt.Println("NextHops:", nextHops)
	<-time.After(2 * time.Second)

	ki, ko, ci, co = "", "123456", "none", "aes"
	hop2 := newHopper("localhost:0", nextHops, ki, ko, ci, co)
	t.Log("Hop2:", hop2.conn.LocalAddr().String(), "->", nextHops, "ki:", ki, "ko:", ko, "ci:", ci, "co:", co)
	go hop2.Start()

	for i := 0; i < 10; i++ {
		<-time.After(time.Millisecond * 100)
		clientConn, err := net.Dial("udp", hop2.conn.LocalAddr().String())
		if err != nil {
			t.Fatalf("Failed to connect to server: %v", err)
		}
		defer clientConn.Close()

		testEcho(t, clientConn)
	}
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randStringBytesRmndr(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}

func testEcho(t *testing.T, clientConn net.Conn) {
	for i := 0; i < 100; i++ {
		msg := randStringBytesRmndr(rand.Intn(mtuLimit - headerSize))
		_, err := clientConn.Write([]byte(msg))
		if err != nil {
			t.Errorf("Failed to send message %d: %v", i, err)
			continue
		}

		clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))

		buffer := make([]byte, mtuLimit)
		n, err := clientConn.Read(buffer)
		if err != nil {
			t.Errorf("Failed to receive response for message %d: %v", i, err)
			continue
		}

		received := string(buffer[:n])
		if received != msg {
			t.Errorf("Expected '%s', but got '%s'", msg, received)
		}
	}
}

func newCrypt(pass []byte, method string) BlockCrypt {
	var block BlockCrypt
	switch method {
	case "aes":
		block, _ = NewAESBlockCrypt(pass)
	case "blowfish":
		block, _ = NewBlowfishBlockCrypt(pass)
	}

	return block
}
