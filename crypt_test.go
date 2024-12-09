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
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"hash/crc32"
	"io"
	mrand "math/rand"
	"testing"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

var key = []byte("testkey")
var pass = pbkdf2.Key(key, []byte("testsalt"), 4096, 32, sha1.New)

func init() {
	mrand.Seed(time.Now().UnixNano())
}

func TestSM4(t *testing.T) {
	bc, err := NewSM4BlockCrypt(pass[:16])
	if err != nil {
		t.Fatal(err)
	}
	cryptTest(t, bc)
}

func TestAES(t *testing.T) {
	bc, err := NewAESBlockCrypt(pass[:32])
	if err != nil {
		t.Fatal(err)
	}
	cryptTest(t, bc)
}

func TestTEA(t *testing.T) {
	bc, err := NewTEABlockCrypt(pass[:16])
	if err != nil {
		t.Fatal(err)
	}
	cryptTest(t, bc)
}

func TestBlowfish(t *testing.T) {
	bc, err := NewBlowfishBlockCrypt(pass[:32])
	if err != nil {
		t.Fatal(err)
	}
	cryptTest(t, bc)
}

func TestCast5(t *testing.T) {
	bc, err := NewCast5BlockCrypt(pass[:16])
	if err != nil {
		t.Fatal(err)
	}
	cryptTest(t, bc)
}

func Test3DES(t *testing.T) {
	bc, err := NewTripleDESBlockCrypt(pass[:24])
	if err != nil {
		t.Fatal(err)
	}
	cryptTest(t, bc)
}

func TestTwofish(t *testing.T) {
	bc, err := NewTwofishBlockCrypt(pass[:32])
	if err != nil {
		t.Fatal(err)
	}
	cryptTest(t, bc)
}

func TestXTEA(t *testing.T) {
	bc, err := NewXTEABlockCrypt(pass[:16])
	if err != nil {
		t.Fatal(err)
	}
	cryptTest(t, bc)
}

func TestSalsa20(t *testing.T) {
	bc, err := NewSalsa20BlockCrypt(pass[:32])
	if err != nil {
		t.Fatal(err)
	}
	cryptTest(t, bc)
}

func cryptTest(t *testing.T, bc BlockCrypt) {
	for i := 0; i < 128; i++ {
		// get a random number between 8 and mtuLimit
		size := mrand.Intn(mtuLimit-8) + 8

		data := make([]byte, size)
		io.ReadFull(rand.Reader, data)
		dec := make([]byte, size)
		enc := make([]byte, size)

		bc.Encrypt(enc, data)
		bc.Decrypt(dec, enc)
		if !bytes.Equal(data, dec) {
			t.Fail()
		}
	}
}

func BenchmarkSM4(b *testing.B) {
	bc, err := NewSM4BlockCrypt(pass[:16])
	if err != nil {
		b.Fatal(err)
	}
	benchCrypt(b, bc)
}

func BenchmarkAES128(b *testing.B) {
	bc, err := NewAESBlockCrypt(pass[:16])
	if err != nil {
		b.Fatal(err)
	}

	benchCrypt(b, bc)
}

func BenchmarkAES192(b *testing.B) {
	bc, err := NewAESBlockCrypt(pass[:24])
	if err != nil {
		b.Fatal(err)
	}

	benchCrypt(b, bc)
}

func BenchmarkAES256(b *testing.B) {
	bc, err := NewAESBlockCrypt(pass[:32])
	if err != nil {
		b.Fatal(err)
	}

	benchCrypt(b, bc)
}

func BenchmarkTEA(b *testing.B) {
	bc, err := NewTEABlockCrypt(pass[:16])
	if err != nil {
		b.Fatal(err)
	}
	benchCrypt(b, bc)
}

func BenchmarkBlowfish(b *testing.B) {
	bc, err := NewBlowfishBlockCrypt(pass[:32])
	if err != nil {
		b.Fatal(err)
	}
	benchCrypt(b, bc)
}

func BenchmarkCast5(b *testing.B) {
	bc, err := NewCast5BlockCrypt(pass[:16])
	if err != nil {
		b.Fatal(err)
	}
	benchCrypt(b, bc)
}

func Benchmark3DES(b *testing.B) {
	bc, err := NewTripleDESBlockCrypt(pass[:24])
	if err != nil {
		b.Fatal(err)
	}
	benchCrypt(b, bc)
}

func BenchmarkTwofish(b *testing.B) {
	bc, err := NewTwofishBlockCrypt(pass[:32])
	if err != nil {
		b.Fatal(err)
	}
	benchCrypt(b, bc)
}

func BenchmarkXTEA(b *testing.B) {
	bc, err := NewXTEABlockCrypt(pass[:16])
	if err != nil {
		b.Fatal(err)
	}
	benchCrypt(b, bc)
}

func BenchmarkSalsa20(b *testing.B) {
	bc, err := NewSalsa20BlockCrypt(pass[:32])
	if err != nil {
		b.Fatal(err)
	}
	benchCrypt(b, bc)
}

func benchCrypt(b *testing.B, bc BlockCrypt) {
	data := make([]byte, mtuLimit)
	io.ReadFull(rand.Reader, data)
	dec := make([]byte, mtuLimit)
	enc := make([]byte, mtuLimit)

	b.ReportAllocs()
	b.SetBytes(int64(len(enc) * 2))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bc.Encrypt(enc, data)
		bc.Decrypt(dec, enc)
	}
}

func BenchmarkCRC32(b *testing.B) {
	content := make([]byte, 1024)
	b.SetBytes(int64(len(content)))
	for i := 0; i < b.N; i++ {
		crc32.ChecksumIEEE(content)
	}
}

func BenchmarkCsprngSystem(b *testing.B) {
	data := make([]byte, md5.Size)
	b.SetBytes(int64(len(data)))

	for i := 0; i < b.N; i++ {
		io.ReadFull(rand.Reader, data)
	}
}

func BenchmarkCsprngMD5(b *testing.B) {
	var data [md5.Size]byte
	b.SetBytes(md5.Size)

	for i := 0; i < b.N; i++ {
		data = md5.Sum(data[:])
	}
}
func BenchmarkCsprngSHA1(b *testing.B) {
	var data [sha1.Size]byte
	b.SetBytes(sha1.Size)

	for i := 0; i < b.N; i++ {
		data = sha1.Sum(data[:])
	}
}
