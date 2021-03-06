package util

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func ParseIntToStringSlice(vals []string) (map[int][]string, error) {
	out := map[int][]string{}
	for _, val := range vals {
		splitVal := strings.SplitN(val, "=", 2)
		if len(splitVal) != 2 {
			return nil, fmt.Errorf("unexpected value %s", val)
		}
		index := CheckAndConvertInt(splitVal[0], "index")
		if _, ok := out[index]; !ok {
			out[index] = []string{splitVal[1]}
		} else {
			out[index] = append(out[index], splitVal[1])
		}
	}
	return out, nil
}

func GetAsBool(input string) (bool, error) {
	switch strings.Trim(input, "\n\t\r\v\f ") {
	case "n":
		fallthrough
	case "no":
		fallthrough
	case "0":
		return false, nil

	case "y":
		fallthrough
	case "yes":
		fallthrough
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("Unknown option for boolean")
	}
}

func YesNoPrompt(msg string) bool {
	if !IsTTY() {
		PrintErrorFatal("not a tty. Did you forget to include -y in your script?")
	}
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("%s ([y]es/[n]o) ", msg)
		if !scanner.Scan() {
			PrintErrorFatal(scanner.Err())
		}
		ask := scanner.Text()
		res, err := GetAsBool(ask)
		if err != nil {
			fmt.Println(err)
			continue
		}
		return res
	}
	panic("should never reach")
}

func OptionListPrompt(msg string, options []string) int {
	if !IsTTY() {
		PrintErrorFatal("not a TTY, failed to give option prompt")
	}
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Println(msg)
		for i, option := range options {
			fmt.Printf("[%d] %s\n", i, option)
		}
		fmt.Printf("\nenter your selection: ")

		if !scanner.Scan() {
			PrintErrorFatal(scanner.Err())
		}
		userResponse := scanner.Text()
		selection, err := strconv.Atoi(userResponse)
		if err != nil {
			InvalidInteger("selection", userResponse, false)
			continue
		}
		if selection >= len(options) || selection < 0 {
			fmt.Println("option does not exist")
			continue
		}
		return selection
	}
	panic("should never reach")
}

func ArgsToJSON(args []string) []interface{} {
	out := []interface{}{}

	for _, arg := range args {
		var conv interface{}
		err := json.Unmarshal([]byte(arg), &conv)
		if err != nil {
			out = append(out, arg)
		} else {
			out = append(out, conv)
		}
	}
	return out
}
