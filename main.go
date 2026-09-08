package main

import (
	"bufio"
	"fmt"
	"hash/fnv"
	"os"
	"strings"
	"sync"
)

type BloomFilter struct {
	bitset []byte
	m      uint64
	k      uint64
	mu     sync.RWMutex
}

func NewBloomFilter(bits uint64, hashes uint64) *BloomFilter {
	byteSize := (bits + 7) / 8

	return &BloomFilter{
		bitset: make([]byte, byteSize),
		m:      bits,
		k:      hashes,
	}
}

func hash1(data []byte) uint64 {
	h := fnv.New64a()
	h.Write(data)
	return h.Sum64()
}

func hash2(data []byte) uint64 {
	h := fnv.New64()
	h.Write(data)
	return h.Sum64()
}

func (bf *BloomFilter) location(hash uint64) (uint64, uint8) {
	byteIndex := hash / 8
	bitIndex := hash % 8
	return byteIndex, uint8(bitIndex)
}

func (bf *BloomFilter) setBit(hash uint64) {
	byteIndex, bitIndex := bf.location(hash)
	bf.bitset[byteIndex] |= 1 << bitIndex
}

func (bf *BloomFilter) getBit(hash uint64) bool {
	byteIndex, bitIndex := bf.location(hash)
	return bf.bitset[byteIndex]&(1<<bitIndex) != 0
}

func (bf *BloomFilter) Add(s string) {
	data := []byte(strings.ToLower(strings.TrimSpace(s)))

	h1 := hash1(data)
	h2 := hash2(data)

	bf.mu.Lock()
	defer bf.mu.Unlock()

	for i := uint64(0); i < bf.k; i++ {
		hash := (h1 + i*h2) % bf.m
		bf.setBit(hash)
	}
}

func (bf *BloomFilter) Contains(s string) bool {
	data := []byte(strings.ToLower(strings.TrimSpace(s)))

	h1 := hash1(data)
	h2 := hash2(data)

	bf.mu.RLock()
	defer bf.mu.RUnlock()

	for i := uint64(0); i < bf.k; i++ {
		hash := (h1 + i*h2) % bf.m
		if !bf.getBit(hash) {
			return false
		}
	}

	return true
}

func main() {

	/*
		Example configuration

		expected items = 10000
		false positive ≈ 1%
	*/

	filter := NewBloomFilter(100000, 7)

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("Enter username: ")

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("input error")
			continue
		}

		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		if filter.Contains(input) {
			fmt.Println("username might already exist")
		} else {
			filter.Add(input)
			fmt.Println("username registered")
		}
	}
}
