## Statement Operators

### statement/remove
Replaces a statement (block or case clause) with a no-op.

**Original**
```go
if n < 0 {
	n = 0
}
n++

n += bar()
```

**Mutated**
```go
if n < 0 {
	n = 0
}
_ = n

n += bar()
```

### statement/removeblock
Removes the block inside error handling code, e.g. err != nil.
Handles err != nil, err == nil, etc. as well as errors whose name is not "err".

**Original**
```go
ok, err := baz()
if err != nil {
	fmt.Println("An error handling block!")
}
```

**Mutated**
```go
ok, err := baz()
if err != nil {
	_ = fmt.Println
}
```

A more complex example:
**Original**
```go
dogErr := err
if 1 != 2 {
	return false
} else if dogErr == nil {
	return true
} else {
	fmt.Println("An error handling block!")
	return
}
```

**Mutated**
```go
dogErr := err
if 1 != 2 {
	return false
} else if dogErr == nil {
	return true
} else {
	_ = fmt.Println
}
```
