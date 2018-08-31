## Distributed Mutation Operators

### distributed/readzero
Adds a line setting the bytes read to zero after a Conn.Read() call.

**Original**
```go
n, err := conn.Read(byte_res)
if n == 0 {
// ...
}
```
**Mutated**
```go
n, err := conn.Read(byte_res)
n = 0
if n == 0 {
// ...
}
```
