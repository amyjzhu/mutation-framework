## Expression Operators

### expression/remove

Replaces a conditional expression with an equivalent nop. For booleans, returns true for `and` operators and false for `or` operators.


**Original**
```go
if i >= 1 && i <= 1 {
	fmt.Println(i)
}
```

**Mutated**
```go
if true && i <= 1 {
    fmt.Println(i)
}
```