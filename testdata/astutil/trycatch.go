package astutil

import (
	"errors"
	"fmt"
)

func foo() bool {

	error1 := fmt.Errorf("fjlsdsf")
	if error1 != nil {
		fmt.Println("An error handling block!")
	}

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

	if 1 == 2 {
		return true
	} else if 1 == 3 {
		return true
	} else {
		return true
	}

	return ok
}
