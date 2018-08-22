package statement

import (
	"errors"
	"fmt"
)

func foo() bool {
	baz := func() (bool, error) {
		return false, errors.New("error")
	}

	ok, err := baz()
	if err != nil {
		fmt.Println("An error handling block!")
	}

	if err == nil {
		return true
	} else {
		fmt.Println("An error handling block!")
	}

	if 1 != 2 {
		return false
	} else if nil != err {
		fmt.Println("An error handling block!")
	}

	if 1 != 2 {
		return false
	} else if err == nil {
		return true
	}

	if 1 != 2 {
		return false
	} else if err == nil {
		return true
	} else {
		fmt.Println("An error handling block!")
	}

	dogErr := err
	if 1 != 2 {
		return false
	} else if dogErr == nil {
		return true
	} else {
		_ = fmt.Println
	}

	if 1 == 2 {
		return true
	} else if 1 == 3 {
		return true
	} else {
		return true
	}

	return ok
}
