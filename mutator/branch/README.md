## Branch Mutation Operators

Mutates various cases and blocks of control-flow statements like `switch`, `if`, `else`. 

### branch/case
**Original**
```go
switch {
	case i == 1:
		fmt.Println(i)
	case i == 2:
		fmt.Println(i * 2)
	default:
		fmt.Println(i * 3)
}
```

**Mutated**

```go
switch {
	case i == 1:
		_, _ = fmt.Println, i
	case i == 2:
		fmt.Println(i * 2)
	default:
		fmt.Println(i * 3)
}
```

### branch/else

**Original**

```go
if i == 1 {
	fmt.Println(i)
} else if i == 2 {
	fmt.Println(i * 2)
} else {
	fmt.Println(i * 3)
}
```


**Mutated**

```go
if i == 1 {
	fmt.Println(i)
} else if i == 2 {
	fmt.Println(i * 2)
} else {
	_, _ = fmt.Println, i
}
```

### branch/if


**Original**
```go
if i == 1 {
	fmt.Println(i)
} else if i == 2 {
	fmt.Println(i * 2)
} else {
	fmt.Println(i * 3)
}
```

**Mutated**

```go
if i == 1 {
	_, _ = fmt.Println, i
} else if i == 2 {
	fmt.Println(i * 2)
} else {
	fmt.Println(i * 3)
}
```



