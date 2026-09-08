# Bloom Filter: Username Registration Demo

A minimal Bloom filter implemented in Go, demonstrated with a CLI that checks whether a username is already taken.

## What this is

A Bloom filter is a probabilistic set. It answers one question : "have I seen this before?" : with two possible results:

- **"No"** : always correct. The item is definitely not in the set.
- **"Maybe"** : could be wrong. The item is probably in the set, but it might be a false positive.

It never produces false negatives. It can produce false positives. That asymmetry is the entire tradeoff: you get O(1) lookups and a fixed memory footprint (independent of item size) in exchange for accepting some rate of "maybe" being wrong.

## How this implementation works

### Structure

```go
type BloomFilter struct {
    bitset []byte
    m      uint64 // total bits in the filter
    k      uint64 // number of hash functions
}
```

`m` is the size of the bit array. `k` is how many bit positions get set per inserted item.

### Hashing

Instead of running `k` independent hash functions (expensive), this uses **double hashing** (Kirsch–Mitzenmacher):

```go
h1 := hash1(data) // FNV-1a
h2 := hash2(data) // FNV-1

for i := 0; i < k; i++ {
    hash := (h1 + i*h2) % m
    setBit(hash) // or getBit(hash)
}
```

Two real hash values (`h1`, `h2`) are combined to simulate `k` independent hashes. This is standard practice : it gives comparable false-positive behavior to `k` distinct hash functions at a fraction of the compute cost.

### Add / Contains

- `Add(s)` : normalizes the string (`ToLower` + `TrimSpace`), computes `k` bit positions, sets them all.
- `Contains(s)` : same normalization, same `k` positions, returns `true` only if *every* one of them is already set.

```go
filter := NewBloomFilter(100_000, 7) // m = 100,000 bits, k = 7

filter.Add("alice")
filter.Contains("alice") // true
filter.Contains("bob")   // false (unless a collision happened)
```

Both are protected by a `sync.RWMutex`, so the type is safe for concurrent use even though the current CLI drives it single-threaded.

### Configuration used here

```
m = 100,000 bits  (~12.2 KB)
k = 7 hash functions
```

Sized for roughly n = 10,000 expected usernames at approximately 1% false-positive rate.

## The math

### False positive formula

```
p = (1 - e^(-k*n/m))^k
```

- `n` : number of items inserted
- `m` : size of the bit array
- `k` : number of hash functions

The false positive rate depends on all three values together, not on `k` alone. More hash functions is not automatically better : see below.

### Optimal k for a given m and n

```
k = (m/n) * ln(2)
```

Example: `m = 8192`, `n = 1000` → `k ≈ (8192/1000) * 0.693 ≈ 5.67` → round to `k = 6`.

### Effect of k at fixed m and n (m=8192, n=1000)

| k  | False positive rate |
|----|----------------------|
| 1  | ~11%                 |
| 2  | ~5%                  |
| 4  | ~2%                  |
| 6  | ~1.6% (near optimal) |
| 10 | worse again          |

Past the optimum, more hash functions set more bits per insert, saturating the bit array faster and raising the false-positive rate. `k` has a sweet spot, not a "more is better" curve.

### Sizing the bit array for a target false-positive rate

```
m ≈ -(n * ln(p)) / (ln(2)^2)
```

Example: for `n = 1,000,000` items at `p = 1%`, you need `m ≈ 9.6 million bits` (~1.2 MB) and `k ≈ 7`.

### Reference points from real systems

| System             | Hash functions (k)        |
|--------------------|----------------------------|
| Bigtable           | ~7                         |
| Cassandra          | ~6                         |
| Redis Bloom filter | configurable (often 6–10)  |

## Running it

```bash
go run main.go
```

```
Enter username: alice
username registered
Enter username: alice
username might already exist
Enter username: bob
username registered
```

## Known limitations

- **No persistence** : state is lost on every restart.
- **No removal** : standard Bloom filters can't delete items; a counting Bloom filter would be needed for that.
- **Fixed m and k** : not configurable at runtime, and no monitoring of actual fill ratio as `n` grows past the sized-for value.
- **EOF handling bug** : on stdin EOF, `ReadString` errors permanently, and the current loop retries forever instead of exiting.

## When to use a Bloom filter for this problem

Good fit for a **pre-check** in front of a real datastore: reject "definitely new" usernames instantly, and only hit the database when the filter says "maybe taken" (to confirm and rule out false positives). Bad fit as the *sole* source of truth, since a false positive here means silently refusing a valid, available username.
